package service

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/bigquery/storage/managedwriter"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"

	"github.com/gabihodoroga/pubsub-to-bigquery/model"
)

type EventSinkBigQuery struct {
	managedStream *managedwriter.ManagedStream
}

func NewEventSinkBigQuery(ctx context.Context, project, dataset, table string) (model.EventSink, error) {

	bqclient, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create managedwriter client")
	}

	perms, err := bqclient.Dataset(dataset).Table(table).IAM().TestPermissions(ctx, []string{
		"bigquery.tables.updateData",
	})

	if err != nil {
		return nil, errors.Wrapf(err,
			"failed to get bigquery table permission permissions, project: %s, dataset: %s, table: %s",
			project,
			dataset,
			table)
	}

	if len(perms) == 0 {
		return nil, fmt.Errorf(
			"required permissions (bigquery.tables.updateData) not found for project: %s, dataset: %s, table: %s",
			project,
			dataset,
			table)
	}

	client, err := managedwriter.NewClient(ctx, project)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create managedwriter client")
	}

	m := &model.Event{}
	descriptorProto := protodesc.ToDescriptorProto(m.ProtoReflect().Descriptor())

	tableName := fmt.Sprintf("projects/%s/datasets/%s/tables/%s", project, dataset, table)
	managedStream, err := client.NewManagedStream(ctx,
		managedwriter.WithDestinationTable(tableName),
		managedwriter.WithType(managedwriter.DefaultStream),
		managedwriter.WithSchemaDescriptor(descriptorProto))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create managedwriter stream")
	}

	// todo: handle connection lost and reconnect
	go func() {
		<-ctx.Done()
		zap.L().Info("EventSinkBigQuery: context done, closing managed stream")
		managedStream.Close()
	}()

	return &EventSinkBigQuery{
		managedStream: managedStream,
	}, nil
}

func (s *EventSinkBigQuery) Save(ctx context.Context, events []*model.Event) error {
	logger := zap.L().With(zap.Any("request_id", ctx.Value(model.ContextKey("request_id"))))
	logger.Debug("EventSinkBigQuery.save: begin request")

	ctx, span := tracer.Start(ctx, "bigquery/save")
	defer span.End()

	// Encode the messages into binary format.
	encoded := make([][]byte, len(events))
	for k, v := range events {
		b, err := proto.Marshal(v)
		if err != nil {
			panic(err)
		}
		encoded[k] = b
	}
	logger.Debug("EventSinkBigQuery.save: begin append rows")
	// Send the rows to the service
	result, err := s.managedStream.AppendRows(ctx, encoded)
	if err != nil {
		return errors.Wrap(err, "failed to append rows")
	}
	logger.Debug("EventSinkBigQuery.save: rows appended, waiting result")
	// Block until the write is complete and return the result.
	_, err = result.GetResult(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to save rows")
	}

	logger.Debug("EventSinkBigQuery.save: rows saved")
	return nil
}
