/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	gogit "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-log.v1"

	"github.com/sniperkit/snk.fork.lookout"
	"github.com/sniperkit/snk.fork.lookout/service/bblfsh"
	"github.com/sniperkit/snk.fork.lookout/service/git"
	"github.com/sniperkit/snk.fork.lookout/util/cli"
	"github.com/sniperkit/snk.fork.lookout/util/grpchelper"
)

type EventCommand struct {
	cli.CommonOptions
	DataServer string `long:"data-server" default:"ipv4://localhost:10301" env:"LOOKOUT_DATA_SERVER" description:"gRPC URL to bind the data server to"`
	Bblfshd    string `long:"bblfshd" default:"ipv4://localhost:9432" env:"LOOKOUT_BBLFSHD" description:"gRPC URL of the Bblfshd server"`
	GitDir     string `long:"git-dir" default:"." env:"GIT_DIR" description:"path to the .git directory to analyze"`
	RevFrom    string `long:"from" default:"HEAD^" description:"name of the base revision for event"`
	RevTo      string `long:"to" default:"HEAD" description:"name of the head revision for event"`
	Args       struct {
		Analyzer string `positional-arg-name:"analyzer" description:"gRPC URL of the analyzer to use"`
	} `positional-args:"yes" required:"yes"`

	repo *gogit.Repository
}

func (c *EventCommand) openRepository() error {
	var err error

	c.repo, err = gogit.PlainOpenWithOptions(c.GitDir, &gogit.PlainOpenOptions{
		DetectDotGit: true,
	})

	return err
}

func (c *EventCommand) resolveRefs() (*lookout.ReferencePointer, *lookout.ReferencePointer, error) {
	baseHash, err := getCommitHashByRev(c.repo, c.RevFrom)
	if err != nil {
		return nil, nil, fmt.Errorf("base revision error: %s", err)
	}

	headHash, err := getCommitHashByRev(c.repo, c.RevTo)
	if err != nil {
		return nil, nil, fmt.Errorf("head revision error: %s", err)
	}

	fullGitPath, err := filepath.Abs(c.GitDir)
	if err != nil {
		return nil, nil, fmt.Errorf("can't resolve full path: %s", err)
	}

	fromRef := lookout.ReferencePointer{
		InternalRepositoryURL: "file://" + fullGitPath,
		ReferenceName:         plumbing.ReferenceName(c.RevFrom),
		Hash:                  baseHash,
	}

	toRef := lookout.ReferencePointer{
		InternalRepositoryURL: "file://" + fullGitPath,
		ReferenceName:         plumbing.ReferenceName(c.RevTo),
		Hash:                  headHash,
	}

	return &fromRef, &toRef, nil
}

type dataService interface {
	lookout.ChangeGetter
	lookout.FileGetter
}

func (c *EventCommand) makeDataServerHandler() (*lookout.DataServerHandler, error) {
	var err error

	var dataService dataService

	loader := git.NewStorerCommitLoader(c.repo.Storer)
	dataService = git.NewService(loader)

	grpcAddr, err := grpchelper.ToGoGrpcAddress(c.Bblfshd)
	if err != nil {
		return nil, fmt.Errorf("Can't resolve bblfsh address '%s': %s", c.Bblfshd, err)
	}
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	bblfshConn, err := grpchelper.DialContext(timeoutCtx, grpcAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Warningf("bblfshd instance could not be found at %s. No UASTs will be available to analyzers. Error: %s", c.Bblfshd, err)
		dataService = &noBblfshService{
			changes: dataService,
			files:   dataService,
		}
	} else {
		dataService = bblfsh.NewService(dataService, dataService, bblfshConn)
	}

	srv := &lookout.DataServerHandler{
		ChangeGetter: dataService,
		FileGetter:   dataService,
	}

	return srv, nil
}

func (c *EventCommand) bindDataServer(srv *lookout.DataServerHandler, serveResult chan error) (*grpc.Server, error) {
	grpcSrv := grpchelper.NewServer()
	lookout.RegisterDataServer(grpcSrv, srv)

	lis, err := grpchelper.Listen(c.DataServer)
	if err != nil {
		return nil, fmt.Errorf("Can't start data server at '%s': %s", c.DataServer, err)
	}

	go func() { serveResult <- grpcSrv.Serve(lis) }()

	return grpcSrv, nil
}

func (c *EventCommand) analyzerClient() (lookout.AnalyzerClient, error) {
	var err error

	grpcAddr, err := grpchelper.ToGoGrpcAddress(c.Args.Analyzer)
	if err != nil {
		return nil, fmt.Errorf("Can't resolve address of analyzer '%s': %s", c.Args.Analyzer, err)
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := grpchelper.DialContext(
		timeoutCtx,
		grpcAddr,
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("Can't connect to analyzer '%s': %s", grpcAddr, err)
	}

	return lookout.NewAnalyzerClient(conn), nil
}

func getCommitHashByRev(r *gogit.Repository, revName string) (string, error) {
	h, err := r.ResolveRevision(plumbing.Revision(revName))
	if err != nil {
		return "", err
	}

	return h.String(), nil
}

type noBblfshService struct {
	changes lookout.ChangeGetter
	files   lookout.FileGetter
}

var _ lookout.ChangeGetter = &noBblfshService{}
var _ lookout.FileGetter = &noBblfshService{}

var errNoBblfsh = errors.New("Data server was started without bbflsh. WantUAST isn't allowed")

func (s *noBblfshService) GetChanges(ctx context.Context, req *lookout.ChangesRequest) (lookout.ChangeScanner, error) {
	if req.WantUAST {
		return nil, errNoBblfsh
	}

	return s.changes.GetChanges(ctx, req)
}

func (s *noBblfshService) GetFiles(ctx context.Context, req *lookout.FilesRequest) (lookout.FileScanner, error) {
	if req.WantUAST {
		return nil, errNoBblfsh
	}

	return s.files.GetFiles(ctx, req)
}
