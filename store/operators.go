/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package store

import (
	"context"

	"github.com/sniperkit/snk.fork.lookout"
	"github.com/sniperkit/snk.fork.lookout/store/models"
)

// EventOperator manages persistence of Events
type EventOperator interface {
	// Save persists Event in a store and returns Status if event was persisted already
	Save(context.Context, lookout.Event) (models.EventStatus, error)
	// UpdateStatus updates Status of event in a store
	UpdateStatus(context.Context, lookout.Event, models.EventStatus) error
}

// CommentOperator manages persistence of Comments
type CommentOperator interface {
	// Save persists Comment in a store
	Save(context.Context, lookout.Event, *lookout.Comment) error
	// Posted checks if a comment was already posted for review
	Posted(context.Context, lookout.Event, *lookout.Comment) (bool, error)
}

// NoopEventOperator satisfies EventOperator interface but does nothing
type NoopEventOperator struct{}

var _ EventOperator = &NoopEventOperator{}

// Save implements EventOperator interface and always returns New status
func (o *NoopEventOperator) Save(context.Context, lookout.Event) (models.EventStatus, error) {
	return models.EventStatusNew, nil
}

// UpdateStatus implements EventOperator interface and does nothing
func (o *NoopEventOperator) UpdateStatus(context.Context, lookout.Event, models.EventStatus) error {
	return nil
}

// NoopCommentOperator satisfies CommentOperator interface but does nothing
type NoopCommentOperator struct{}

var _ CommentOperator = &NoopCommentOperator{}

// Save implements EventOperator interface and does nothing
func (o *NoopCommentOperator) Save(context.Context, lookout.Event, *lookout.Comment) error {
	return nil
}

// Posted implements EventOperator interface and always returns false
func (o *NoopCommentOperator) Posted(context.Context, lookout.Event, *lookout.Comment) (bool, error) {
	return false, nil
}
