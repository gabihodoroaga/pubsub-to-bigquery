package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/dchest/uniuri"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gabihodoroga/pubsub-to-bigquery/model"
)

// EventHandlerPubsub implements the EventHandler using Pub/Sub
type EventHandlerPubsub struct {
	host         string
	project      string
	subscription string
	sink         model.EventSink
	stats        *model.HandlerStats
}

func NewEventHandlerPubsub(host, project, subscription string, sink model.EventSink) (model.EventHandler, error) {
	return &EventHandlerPubsub{
		host:         host,
		project:      project,
		subscription: subscription,
		sink:         sink,
		stats:        &model.HandlerStats{},
	}, nil
}

// Start implements model.EventHandler and start listening for events
func (c *EventHandlerPubsub) Start(ctx context.Context) error {

	var client *pubsub.Client
	var err error

	if c.host != "" {
		// This is mainly used for testing
		conn, err := grpc.Dial(c.host, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return err
		}
		// Use the connection when creating a pubsub client.
		client, err = pubsub.NewClient(ctx, "project", option.WithGRPCConn(conn))
		if err != nil {
			return err
		}
	} else {
		client, err = pubsub.NewClient(ctx, c.project)

		if err != nil {
			return errors.Wrap(err, "failed to create PubSub client")
		}
	}

	sub := client.Subscription(c.subscription)

	if c.host == "" {
		perms, err := sub.IAM().TestPermissions(ctx, []string{
			"pubsub.subscriptions.consume",
		})

		if err != nil {
			return errors.Wrapf(err,
				"failed to get the subscription permissions, project %s, subscription %s",
				c.project,
				c.subscription)
		}

		if len(perms) == 0 {
			return fmt.Errorf(
				"required permissions (pubsub.subscriptions.consume) not found for project %s, subscription %s",
				c.project,
				c.subscription)
		}
	}

	go func() {
		for {
			zap.L().Sugar().Infof("begin receive messages from subscription %s.", sub.String())
			err = sub.Receive(ctx, func(msgCtx context.Context, msg *pubsub.Message) {

				atomic.AddInt64(&c.stats.Received, 1)
				requestID := uniuri.NewLen(10)
				logger := zap.L().With(zap.Any("request_id", requestID))
				newCtx := context.WithValue(msgCtx, model.ContextKey("request_id"), requestID)
				newCtx, span := tracer.Start(newCtx, "pubsub/receive")
				defer span.End()

				logger.Sugar().Debugf("pubsubFunc: got message with id %s, data: %s", msg.ID, string(msg.Data))
				err := c.handleMessage(newCtx, msg.Data)
				if err != nil {
					logger.Error(fmt.Sprintf("pubsubFunc: failed to process message with id %s", msg.ID), zap.Error(err))
					msg.Nack()
					atomic.AddInt64(&c.stats.Errors, 1)
				} else {
					logger.Sugar().Debugf("pubsubFunc: message with id %s acknowledged", msg.ID)
					msg.Ack()
					atomic.AddInt64(&c.stats.Success, 1)
				}
			})
			zap.L().Sugar().Infof("pubsub receive exit for subscription %s", sub.String())

			if err != nil {
				zap.L().Error(fmt.Sprintf("pubsub receive error for subscription %s, receive will be retried in 2 seconds", sub.String()), zap.Error(err))
				time.Sleep(time.Second * 2)
				continue
			}
			// if no error is received then the context has been canceled and we just exist
			zap.L().Sugar().Infof("receive done on subscription %s. No messages will be processed.", sub.String())
			client.Close()
			return
		}
	}()

	return nil
}

// Stats implements model.EventHandler
func (c *EventHandlerPubsub) Stats(ctx context.Context) (model.HandlerStats, error) {
	return *c.stats, nil
}

func (c *EventHandlerPubsub) handleMessage(ctx context.Context, data []byte) error {
	logger := zap.L().With(zap.Any("request_id", ctx.Value(model.ContextKey("request_id"))))
	logger.Debug("handleMessage: begin request")

	ctx, span := tracer.Start(ctx, "pubsub/handleMessage")
	defer span.End()

	message := &model.Event{}
	if err := json.Unmarshal(data, message); err != nil {
		return err
	}
	logger.Sugar().Debugf("handleMessage: message parsed: %v", message)

	// Do some processing here

	// Bundle the messages together
	events := []*model.Event{message}

	// send the message to the sing
	err := c.sink.Save(ctx, events)
	if err != nil {
		return errors.Wrap(err, "failed to send message to sink")
	}

	logger.Sugar().Debugf("handleMessage: message saved")
	return nil
}
