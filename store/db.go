/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package store

import (
	"context"
	"fmt"

	kallax "gopkg.in/src-d/go-kallax.v1"
	log "gopkg.in/src-d/go-log.v1"

	"github.com/sniperkit/snk.fork.lookout"
	"github.com/sniperkit/snk.fork.lookout/store/models"
)

// DBEventOperator operates on event database store
type DBEventOperator struct {
	reviewsStore *models.ReviewEventStore
	pushStore    *models.PushEventStore
}

// NewDBEventOperator creates new DBEventOperator using kallax as storage
func NewDBEventOperator(r *models.ReviewEventStore, p *models.PushEventStore) *DBEventOperator {
	return &DBEventOperator{r, p}
}

var _ EventOperator = &DBEventOperator{}

// Save implements EventOperator interface
func (o *DBEventOperator) Save(ctx context.Context, e lookout.Event) (models.EventStatus, error) {
	switch ev := e.(type) {
	case *lookout.ReviewEvent:
		return o.saveReview(ctx, ev)
	case *lookout.PushEvent:
		return o.savePush(ctx, ev)
	default:
		log.Debugf("ignoring unsupported event: %s", ev)
	}

	return models.EventStatusNew, nil
}

// UpdateStatus implements EventOperator interface
func (o *DBEventOperator) UpdateStatus(ctx context.Context, e lookout.Event, status models.EventStatus) error {
	switch ev := e.(type) {
	case *lookout.ReviewEvent:
		return o.updateReviewStatus(ctx, ev, status)
	case *lookout.PushEvent:
		return o.updatePushStatus(ctx, ev, status)
	default:
		log.Debugf("ignoring unsupported event: %s", ev)
		return nil
	}
}

func (o *DBEventOperator) saveReview(ctx context.Context, e *lookout.ReviewEvent) (models.EventStatus, error) {
	m, err := o.getReview(ctx, e)
	if err == kallax.ErrNotFound {
		return models.EventStatusNew, o.reviewsStore.Insert(models.NewReviewEvent(e))
	}
	if err != nil {
		return models.EventStatusNew, err
	}

	status := models.EventStatusNew
	if m.Status != "" {
		status = m.Status
	}
	return status, nil
}

func (o *DBEventOperator) updateReviewStatus(ctx context.Context, e *lookout.ReviewEvent, s models.EventStatus) error {
	m, err := o.getReview(ctx, e)
	if err != nil {
		return err
	}

	m.Status = s

	_, err = o.reviewsStore.Update(m, models.Schema.ReviewEvent.Status)

	return err
}

func (o *DBEventOperator) getReview(ctx context.Context, e *lookout.ReviewEvent) (*models.ReviewEvent, error) {
	q := models.NewReviewEventQuery().
		FindByProvider(e.Provider).
		FindByInternalID(e.InternalID)

	return o.reviewsStore.FindOne(q)
}

func (o *DBEventOperator) savePush(ctx context.Context, e *lookout.PushEvent) (models.EventStatus, error) {
	m, err := o.getPush(ctx, e)
	if err == kallax.ErrNotFound {
		return models.EventStatusNew, o.pushStore.Insert(models.NewPushEvent(e))
	}
	if err != nil {
		return models.EventStatusNew, err
	}

	status := models.EventStatusNew
	if m.Status != "" {
		status = m.Status
	}
	return status, nil
}

func (o *DBEventOperator) updatePushStatus(ctx context.Context, e *lookout.PushEvent, s models.EventStatus) error {
	m, err := o.getPush(ctx, e)
	if err != nil {
		return err
	}

	m.Status = s

	_, err = o.pushStore.Update(m, models.Schema.PushEvent.Status)

	return err
}

func (o *DBEventOperator) getPush(ctx context.Context, e *lookout.PushEvent) (*models.PushEvent, error) {
	q := models.NewPushEventQuery().
		FindByProvider(e.Provider).
		FindByInternalID(e.InternalID)

	return o.pushStore.FindOne(q)
}

// DBCommentOperator operates on comments database store
type DBCommentOperator struct {
	store        *models.CommentStore
	reviewsStore *models.ReviewEventStore
}

// NewDBCommentOperator creates new DBCommentOperator using kallax as storage
func NewDBCommentOperator(c *models.CommentStore, r *models.ReviewEventStore) *DBCommentOperator {
	return &DBCommentOperator{c, r}
}

var _ CommentOperator = &DBCommentOperator{}

// Save implements EventOperator interface
func (o *DBCommentOperator) Save(ctx context.Context, e lookout.Event, c *lookout.Comment) error {
	ev, ok := e.(*lookout.ReviewEvent)
	if !ok {
		return fmt.Errorf("comments can belong only to review event but %v is given", e.Type())
	}

	return o.save(ctx, ev, c)
}

// Posted implements EventOperator interface
func (o *DBCommentOperator) Posted(ctx context.Context, e lookout.Event, c *lookout.Comment) (bool, error) {
	ev, ok := e.(*lookout.ReviewEvent)
	if !ok {
		return false, fmt.Errorf("comments can belong only to review event but %v is given", e.Type())
	}

	return o.posted(ctx, ev, c)
}

func (o *DBCommentOperator) save(ctx context.Context, e *lookout.ReviewEvent, c *lookout.Comment) error {
	q := models.NewReviewEventQuery().
		FindByProvider(e.Provider).
		FindByInternalID(e.InternalID)

	r, err := o.reviewsStore.FindOne(q)
	if err != nil {
		return err
	}

	m := models.NewComment(r, c)
	_, err = o.store.Save(m)
	return err
}

func (o *DBCommentOperator) posted(ctx context.Context, e *lookout.ReviewEvent, c *lookout.Comment) (bool, error) {
	// FIXME(@max): maybe we should use joins here, not sure how to do it with kallax

	reviewIdsQ := models.NewReviewEventQuery().
		FindByProvider(e.Provider).
		FindByRepositoryID(kallax.Eq, e.RepositoryID).
		FindByNumber(kallax.Eq, e.Number).
		Select(models.Schema.ReviewEvent.ID)

	reviews, err := o.reviewsStore.FindAll(reviewIdsQ)
	if err != nil {
		return false, err
	}

	reviewIds := make([]interface{}, len(reviews))
	for i, r := range reviews {
		reviewIds[i] = r.ID
	}

	q := models.NewCommentQuery().
		Where(kallax.In(models.Schema.Comment.ReviewEventFK, reviewIds...)).
		FindByFile(c.File).
		FindByLine(kallax.Eq, c.Line).
		FindByText(c.Text)

	count, err := o.store.Count(q)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
