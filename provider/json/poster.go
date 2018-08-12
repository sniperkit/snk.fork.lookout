/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package json

import (
	"context"
	"encoding/json"
	"io"

	"gopkg.in/src-d/go-log.v1"

	"github.com/sniperkit/snk.fork.lookout"
)

// Poster prints json comments to stdout
type Poster struct {
	writer io.Writer
	enc    *json.Encoder
}

var _ lookout.Poster = &Poster{}

// NewPoster creates a new json poster for stdout
func NewPoster(w io.Writer) *Poster {
	return &Poster{
		writer: w,
		enc:    json.NewEncoder(w),
	}
}

// Post prints json comments to sdtout
func (p *Poster) Post(ctx context.Context, e lookout.Event,
	aCommentsList []lookout.AnalyzerComments) error {

	for _, a := range aCommentsList {
		for _, c := range a.Comments {
			if err := p.enc.Encode(c); err != nil {
				return err
			}
		}
	}

	return nil
}

// Status prints the new status to the log
func (p *Poster) Status(ctx context.Context, e lookout.Event,
	status lookout.AnalysisStatus) error {

	log.With(log.Fields{"status": status}).Infof("New status")
	return nil
}
