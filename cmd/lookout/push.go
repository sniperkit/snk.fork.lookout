/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package main

import (
	"context"
	"time"

	uuid "github.com/satori/go.uuid"
	gogit "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	log "gopkg.in/src-d/go-log.v1"

	"github.com/sniperkit/snk.fork.lookout"
	"github.com/sniperkit/snk.fork.lookout/server"
	"github.com/sniperkit/snk.fork.lookout/store"
)

func init() {
	if _, err := app.AddCommand("push", "provides simple data server and triggers analyzer", "",
		&PushCommand{}); err != nil {
		panic(err)
	}
}

type PushCommand struct {
	EventCommand
}

func (c *PushCommand) Execute(args []string) error {
	if err := c.openRepository(); err != nil {
		return err
	}

	fromRef, toRef, err := c.resolveRefs()
	if err != nil {
		return err
	}

	dataSrv, err := c.makeDataServerHandler()
	if err != nil {
		return err
	}

	serveResult := make(chan error)
	grpcSrv, err := c.bindDataServer(dataSrv, serveResult)
	if err != nil {
		return err
	}

	client, err := c.analyzerClient()
	if err != nil {
		return err
	}

	srv := server.NewServer(nil, &LogPoster{log.DefaultLogger}, dataSrv.FileGetter, map[string]lookout.Analyzer{
		"test-analyzes": lookout.Analyzer{
			Client: client,
		},
	}, &store.NoopEventOperator{}, &store.NoopCommentOperator{})

	log, err := c.repo.Log(&gogit.LogOptions{From: plumbing.NewHash(toRef.Hash)})
	var commits uint32
	for {
		commit, err := log.Next()
		if err != nil {
			return err
		}
		if commit.Hash.String() == fromRef.Hash {
			break
		}
		commits++
	}

	err = srv.HandlePush(context.TODO(), &lookout.PushEvent{
		InternalID: uuid.NewV4().String(),
		CreatedAt:  time.Now(),
		Commits:    commits,
		CommitRevision: lookout.CommitRevision{
			Base: *fromRef,
			Head: *toRef,
		}})

	if err != nil {
		return err
	}

	grpcSrv.GracefulStop()
	return <-serveResult
}
