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
	log "gopkg.in/src-d/go-log.v1"

	"github.com/sniperkit/snk.fork.lookout"
	"github.com/sniperkit/snk.fork.lookout/server"
	"github.com/sniperkit/snk.fork.lookout/store"
)

func init() {
	if _, err := app.AddCommand("review", "provides simple data server and triggers analyzer", "",
		&ReviewCommand{}); err != nil {
		panic(err)
	}
}

type ReviewCommand struct {
	EventCommand
}

func (c *ReviewCommand) Execute(args []string) error {
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

	err = srv.HandleReview(context.TODO(), &lookout.ReviewEvent{
		InternalID:  uuid.NewV4().String(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		IsMergeable: true,
		Source:      *toRef,
		Merge:       *toRef,
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
