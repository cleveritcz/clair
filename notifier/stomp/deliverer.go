package stomp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	gostomp "github.com/go-stomp/stomp"
	"github.com/google/uuid"

	clairerror "github.com/quay/clair/v4/clair-error"
	"github.com/quay/clair/v4/config"
	"github.com/quay/clair/v4/notifier"
)

// Deliverer is a STOMP deliverer which publishes a notifier.Callback to the
// the broker.
type Deliverer struct {
	callback    *url.URL
	destination string
	fo          failOver
	rollup      int
}

func New(conf *config.STOMP) (*Deliverer, error) {
	var d Deliverer
	if err := d.load(conf); err != nil {
		return nil, err
	}
	return &d, nil
}

func (d *Deliverer) load(cfg *config.STOMP) error {
	var err error
	if cfg.TLS != nil {
		d.fo.tls, err = cfg.TLS.Config()
		if err != nil {
			return err
		}
	}
	if !cfg.Direct {
		d.callback, err = url.Parse(cfg.Callback)
		if err != nil {
			return err
		}
	}

	d.fo.uris = make([]string, len(cfg.URIs))
	copy(d.fo.uris, cfg.URIs)
	d.destination = cfg.Destination
	d.rollup = cfg.Rollup
	return nil
}

func (d *Deliverer) Name() string {
	return fmt.Sprintf("stomp-%s", d.destination)
}

func (d *Deliverer) Deliver(ctx context.Context, nID uuid.UUID) error {
	conn, err := d.fo.Connection(ctx)
	if err != nil {
		return &clairerror.ErrDeliveryFailed{err}
	}
	defer conn.Disconnect()

	u, err := d.callback.Parse(nID.String())
	if err != nil {
		return err
	}

	cb := notifier.Callback{
		NotificationID: nID,
		Callback:       *u,
	}
	b, err := json.Marshal(&cb)
	if err != nil {
		return &clairerror.ErrDeliveryFailed{err}
	}

	err = conn.Send(d.destination, "application/json", b, gostomp.SendOpt.Receipt)
	if err != nil {
		return &clairerror.ErrDeliveryFailed{err}
	}
	return nil
}
