/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/golang-migrate/migrate"
	"github.com/gregjones/httpcache/diskcache"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/src-d/go-log.v1"
	yaml "gopkg.in/yaml.v2"

	"github.com/sniperkit/snk.fork.lookout"
	"github.com/sniperkit/snk.fork.lookout/provider/github"
	"github.com/sniperkit/snk.fork.lookout/provider/json"
	"github.com/sniperkit/snk.fork.lookout/server"
	"github.com/sniperkit/snk.fork.lookout/service/bblfsh"
	"github.com/sniperkit/snk.fork.lookout/service/git"
	"github.com/sniperkit/snk.fork.lookout/store"
	"github.com/sniperkit/snk.fork.lookout/store/models"
	"github.com/sniperkit/snk.fork.lookout/util/cache"
	"github.com/sniperkit/snk.fork.lookout/util/cli"
	"github.com/sniperkit/snk.fork.lookout/util/grpchelper"
)

func init() {
	if _, err := app.AddCommand("serve", "run server", "",
		&ServeCommand{}); err != nil {
		panic(err)
	}
}

type ServeCommand struct {
	cli.CommonOptions
	cli.DBOptions
	ConfigFile  string `long:"config" short:"c" default:"config.yml" env:"LOOKOUT_CONFIG_FILE" description:"path to configuration file"`
	GithubUser  string `long:"github-user" env:"GITHUB_USER" description:"user for the GitHub API"`
	GithubToken string `long:"github-token" env:"GITHUB_TOKEN" description:"access token for the GitHub API"`
	DataServer  string `long:"data-server" default:"ipv4://localhost:10301" env:"LOOKOUT_DATA_SERVER" description:"gRPC URL to bind the data server to"`
	Bblfshd     string `long:"bblfshd" default:"ipv4://localhost:9432" env:"LOOKOUT_BBLFSHD" description:"gRPC URL of the Bblfshd server"`
	DryRun      bool   `long:"dry-run" env:"LOOKOUT_DRY_RUN" description:"analyze repositories and log the result without posting code reviews to GitHub"`
	Library     string `long:"library" default:"/tmp/lookout" env:"LOOKOUT_LIBRARY" description:"path to the lookout library"`
	Provider    string `long:"provider" default:"github" env:"LOOKOUT_PROVIDER" description:"provider name: github, json"`
	Positional  struct {
		Repository string `positional-arg-name:"repository"`
	} `positional-args:"yes" required:"yes"`

	analyzers map[string]lookout.AnalyzerClient
	pool      *github.ClientPool
}

// Config holds the main configuration
type Config struct {
	server.Config `yaml:",inline"`
	Providers     struct {
		Github github.ProviderConfig
	}
	Repositories []RepoConfig
}

// RepoConfig holds configuration for repository, support only github provider
type RepoConfig struct {
	URL  string
	Auth github.UserToken
}

func (c *ServeCommand) Execute(args []string) error {
	var conf Config
	configData, err := ioutil.ReadFile(c.ConfigFile)
	if err != nil {
		return fmt.Errorf("Can't open configuration file: %s", err)
	}
	if err := yaml.Unmarshal([]byte(configData), &conf); err != nil {
		return fmt.Errorf("Can't parse configuration file: %s", err)
	}

	dataHandler, err := c.initDataHadler()
	if err != nil {
		return err
	}

	if err := c.startServer(dataHandler); err != nil {
		return err
	}

	analyzers := make(map[string]lookout.Analyzer)
	for _, aConf := range conf.Analyzers {
		if aConf.Disabled {
			continue
		}
		client, err := c.startAnalyzer(aConf)
		if err != nil {
			return err
		}
		analyzers[aConf.Name] = lookout.Analyzer{
			Client: client,
			Config: aConf,
		}
	}

	err = c.initProvider(conf)
	if err != nil {
		return err
	}

	poster, err := c.initPoster(conf)
	if err != nil {
		return err
	}

	watcher, err := c.initWatcher(conf)
	if err != nil {
		return err
	}

	db, err := c.initDB()
	if err != nil {
		return err
	}

	reviewStore := models.NewReviewEventStore(db)
	eventOp := store.NewDBEventOperator(
		reviewStore,
		models.NewPushEventStore(db),
	)
	commentsOp := store.NewDBCommentOperator(
		models.NewCommentStore(db),
		reviewStore,
	)

	ctx := context.Background()
	return server.NewServer(watcher, poster, dataHandler.FileGetter, analyzers, eventOp, commentsOp).Run(ctx)
}

func (c *ServeCommand) initProvider(conf Config) error {
	switch c.Provider {
	case github.Provider:
		var emptyToken github.UserToken

		urls := strings.Split(c.Positional.Repository, ",")
		urlTokens := make(map[string]github.UserToken, len(urls))
		configURLTokens := make(map[string]github.UserToken, len(conf.Repositories))
		for _, repo := range conf.Repositories {
			if repo.Auth != emptyToken {
				configURLTokens[repo.URL] = repo.Auth
			}
		}

		for _, url := range urls {
			token, ok := configURLTokens[url]
			if !ok {
				log.Infof("use default token for repository %s", url)
			}
			urlTokens[url] = token
		}

		cache := cache.NewValidableCache(diskcache.New("/tmp/github"))
		pool, err := github.NewClientPoolFromTokens(urlTokens, github.UserToken{
			User:  c.GithubUser,
			Token: c.GithubToken,
		}, cache)
		if err != nil {
			return err
		}

		c.pool = pool
	}

	return nil
}

func (c *ServeCommand) initPoster(conf Config) (lookout.Poster, error) {
	if c.DryRun {
		return &LogPoster{log.DefaultLogger}, nil
	}

	switch c.Provider {
	case github.Provider:
		return github.NewPoster(c.pool, conf.Providers.Github), nil
	case json.Provider:
		return json.NewPoster(os.Stdout), nil
	default:
		return nil, fmt.Errorf("provider %s not supported", c.Provider)
	}
}

func (c *ServeCommand) initWatcher(conf Config) (lookout.Watcher, error) {
	switch c.Provider {
	case github.Provider:
		watcher, err := github.NewWatcher(c.pool, &lookout.WatchOptions{
			URLs: strings.Split(c.Positional.Repository, ","),
		})
		if err != nil {
			return nil, err
		}

		return watcher, nil
	case json.Provider:
		return json.NewWatcher(os.Stdin, &lookout.WatchOptions{})
	default:
		return nil, fmt.Errorf("provider %s not supported", c.Provider)
	}
}

func (c *ServeCommand) startAnalyzer(conf lookout.AnalyzerConfig) (lookout.AnalyzerClient, error) {
	addr, err := grpchelper.ToGoGrpcAddress(conf.Addr)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	conn, err := grpchelper.DialContext(ctx, addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	go grpchelper.LogConnStatusChanges(ctx, log.With(log.Fields{
		"analyzer": conf.Name,
		"addr":     conf.Addr,
	}), conn)

	return lookout.NewAnalyzerClient(conn), nil
}

func (c *ServeCommand) initDataHadler() (*lookout.DataServerHandler, error) {
	var err error
	c.Bblfshd, err = grpchelper.ToGoGrpcAddress(c.Bblfshd)
	if err != nil {
		return nil, err
	}

	bblfshConn, err := grpchelper.DialContext(context.Background(), c.Bblfshd, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	lib := git.NewLibrary(osfs.New(c.Library))
	sync := git.NewSyncer(lib)
	loader := git.NewLibraryCommitLoader(lib, sync)

	gitService := git.NewService(loader)
	bblfshService := bblfsh.NewService(gitService, gitService, bblfshConn)

	srv := &lookout.DataServerHandler{
		ChangeGetter: bblfshService,
		FileGetter:   bblfshService,
	}

	return srv, nil
}

func (c *ServeCommand) startServer(srv *lookout.DataServerHandler) error {
	grpcSrv := grpchelper.NewServer()
	lookout.RegisterDataServer(grpcSrv, srv)
	lis, err := grpchelper.Listen(c.DataServer)
	if err != nil {
		return err
	}

	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			log.Errorf(err, "data server failed")
		}
	}()
	return nil
}

func (c *ServeCommand) initDB() (*sql.DB, error) {
	db, err := sql.Open("postgres", c.DB)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	m, err := store.NewMigrateInstance(db)
	if err != nil {
		return nil, err
	}

	dbVersion, _, err := m.Version()

	// The DB is not initialized
	if err == migrate.ErrNilVersion {
		return nil, fmt.Errorf("the DB is empty, it needs to be initialized with the 'lookout migrate' command")
	}

	if err != nil {
		return nil, err
	}

	maxVersion, err := store.MaxMigrateVersion()
	if err != nil {
		return nil, err
	}

	if dbVersion != maxVersion {
		return nil, fmt.Errorf(
			"database version mismatch. Current version is %v, but this binary (version %s, built on %s) needs version %v. "+
				"Use 'lookout migrate' to upgrade your database", dbVersion, version, build, maxVersion)
	}

	log.Debugf("the DB version is up to date, %v", dbVersion)
	log.Infof("connection with the DB established")
	return db, nil
}

type LogPoster struct {
	Log log.Logger
}

func (p *LogPoster) Post(ctx context.Context, e lookout.Event,
	aCommentsList []lookout.AnalyzerComments) error {
	for _, aComments := range aCommentsList {
		for _, c := range aComments.Comments {
			logger := p.Log.With(log.Fields{
				"text": c.Text,
			})
			if c.File == "" {
				logger.Infof("global comment")
				continue
			}

			logger = logger.With(log.Fields{"file": c.File})
			if c.Line == 0 {
				logger.Infof("file comment")
				continue
			}

			logger.With(log.Fields{"line": c.Line}).Infof("line comment")
		}
	}

	return nil
}

func (p *LogPoster) Status(ctx context.Context, e lookout.Event,
	status lookout.AnalysisStatus) error {
	p.Log.Infof("status: %s", status)
	return nil
}
