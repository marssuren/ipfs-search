package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	opensearchapi "github.com/opensearch-project/opensearch-go/opensearchapi"
	opensearchutil "github.com/opensearch-project/opensearch-go/opensearchutil"

	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"

	"github.com/ipfs-search/ipfs-search/components/index"
)

// Index wraps an Elasticsearch index to store documents
type Index struct {
	cfg *Config
	c   *Client
}

// New returns a new index.
func New(client *Client, cfg *Config) index.Index {
	if client == nil {
		panic("Index.New Client cannot be nil.")
	}

	if cfg == nil {
		panic("Index.New Config cannot be nil.")
	}

	index := &Index{
		c:   client,
		cfg: cfg,
	}

	return index
}

// String returns the name of the index, for convenient logging.
func (i *Index) String() string {
	return i.cfg.Name
}

// index wraps BulkIndexer.Add().
func (i *Index) index(
	ctx context.Context,
	action string,
	id string,
	properties interface{},
) error {
	var body io.Reader
	if properties != nil {
		body = opensearchutil.NewJSONReader(properties)
	}

	item := opensearchutil.BulkIndexerItem{
		Index:      i.cfg.Name,
		Action:     action,
		Body:       body,
		DocumentID: id,
	}

	return i.c.bulkIndexer.Add(ctx, item)
}

// Index a document's properties, identified by id
func (i *Index) Index(ctx context.Context, id string, properties interface{}) error {
	ctx, span := i.c.Tracer.Start(ctx, "index.elasticsearch.Index")
	defer span.End()

	if err := i.index(ctx, "create", id, properties); err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return err
	}

	return nil
}

// Update a document's properties, given id
func (i *Index) Update(ctx context.Context, id string, properties interface{}) error {
	ctx, span := i.c.Tracer.Start(ctx, "index.elasticsearch.Update")
	defer span.End()

	if err := i.index(ctx, "update", id, properties); err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return err
	}

	return nil
}

// Delete item from index
func (i *Index) Delete(ctx context.Context, id string) error {
	ctx, span := i.c.Tracer.Start(ctx, "index.elasticsearch.Delete")
	defer span.End()

	if err := i.index(ctx, "delete", id, nil); err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return err
	}

	return nil
}

// Get retreives `fields` from document with `id` from the index, returning:
// - (true, decoding_error) if found (decoding error set when errors in json)
// - (false, nil) when not found
// - (false, error) otherwise
func (i *Index) Get(ctx context.Context, id string, dst interface{}, fields ...string) (bool, error) {
	ctx, span := i.c.Tracer.Start(ctx, "index.elasticsearch.Get")
	defer span.End()

	req := opensearchapi.GetRequest{
		Index:          i.cfg.Name,
		DocumentID:     id,
		SourceIncludes: fields,
		Realtime:       &[]bool{true}[0],
		Preference:     "_local",
	}

	res, err := req.Do(ctx, i.c.searchClient)

	// Handle connection errors
	if err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return false, err
	}

	defer res.Body.Close()

	// Decode body
	response := struct {
		Found  bool            `json:"found"`
		Source json.RawMessage `json:"_source"`
	}{}

	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&response)
	if err != nil {
		err = fmt.Errorf("error decoding body: %w", err)
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return false, err
	}

	if response.Found {
		// Decode source
		err = json.Unmarshal(response.Source, dst)
		if err != nil {
			err = fmt.Errorf("error decoding source: %w", err)
			span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
			return false, err
		}

		return true, nil
	}

	if res.StatusCode != 404 {
		// 404's do not signify an error, other status codes do.
		err = fmt.Errorf("unexpected status from backend: %s", res.Status())
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
	}

	return false, err
}

// Close indexer.
func (i *Index) Close(ctx context.Context) error {
	// Close waits until all added items are flushed and closes the bulk indexer.
	return i.c.bulkIndexer.Close(ctx)
}

// Compile-time assurance that implementation satisfies interface.
var _ index.Index = &Index{}
