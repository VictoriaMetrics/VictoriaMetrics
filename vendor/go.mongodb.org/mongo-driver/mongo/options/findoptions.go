// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package options

import (
	"time"
)

// FindOptions represents options that can be used to configure a Find operation.
type FindOptions struct {
	// If true, an operation on a sharded cluster can return partial results if some shards are down rather than
	// returning an error. The default value is false.
	AllowPartialResults *bool

	// The maximum number of documents to be included in each batch returned by the server.
	BatchSize *int32

	// Specifies a collation to use for string comparisons during the operation. This option is only valid for MongoDB
	// versions >= 3.4. For previous server versions, the driver will return an error if this option is used. The
	// default value is nil, which means the default collation of the collection will be used.
	Collation *Collation

	// A string that will be included in server logs, profiling logs, and currentOp queries to help trace the operation.
	// The default is the empty string, which means that no comment will be included in the logs.
	Comment *string

	// Specifies the type of cursor that should be created for the operation. The default is NonTailable, which means
	// that the cursor will be closed by the server when the last batch of documents is retrieved.
	CursorType *CursorType

	// The index to use for the aggregation. This should either be the index name as a string or the index specification
	// as a document. The default value is nil, which means that no hint will be sent.
	Hint interface{}

	// The maximum number of documents to return. The default value is 0, which means that all documents matching the
	// filter will be returned. A negative limit specifies that the resulting documents should be returned in a single
	// batch. The default value is 0.
	Limit *int64

	// A document specifying the exclusive upper bound for a specific index. The default value is nil, which means that
	// there is no maximum value.
	Max interface{}

	// The maximum amount of time that the server should wait for new documents to satisfy a tailable cursor query.
	// This option is only valid for tailable await cursors (see the CursorType option for more information) and
	// MongoDB versions >= 3.2. For other cursor types or previous server versions, this option is ignored.
	MaxAwaitTime *time.Duration

	// The maximum amount of time that the query can run on the server. The default value is nil, meaning that there
	// is no time limit for query execution.
	MaxTime *time.Duration

	// A document specifying the inclusive lower bound for a specific index. The default value is 0, which means that
	// there is no minimum value.
	Min interface{}

	// If true, the cursor created by the operation will not timeout after a period of inactivity. The default value
	// is false.
	NoCursorTimeout *bool

	// This option is for internal replication use only and should not be set.
	OplogReplay *bool

	// A document describing which fields will be included in the documents returned by the operation. The default value
	// is nil, which means all fields will be included.
	Projection interface{}

	// If true, the documents returned by the operation will only contain fields corresponding to the index used. The
	// default value is false.
	ReturnKey *bool

	// If true, a $recordId field with a record identifier will be included in the documents returned by the operation.
	// The default value is false.
	ShowRecordID *bool

	// The number of documents to skip before adding documents to the result. The default value is 0.
	Skip *int64

	// If true, the cursor will not return a document more than once because of an intervening write operation. This
	// option has been deprecated in MongoDB version 4.0. The default value is false.
	Snapshot *bool

	// A document specifying the order in which documents should be returned.
	Sort interface{}
}

// Find creates a new FindOptions instance.
func Find() *FindOptions {
	return &FindOptions{}
}

// SetAllowPartialResults sets the value for the AllowPartialResults field.
func (f *FindOptions) SetAllowPartialResults(b bool) *FindOptions {
	f.AllowPartialResults = &b
	return f
}

// SetBatchSize sets the value for the BatchSize field.
func (f *FindOptions) SetBatchSize(i int32) *FindOptions {
	f.BatchSize = &i
	return f
}

// SetCollation sets the value for the Collation field.
func (f *FindOptions) SetCollation(collation *Collation) *FindOptions {
	f.Collation = collation
	return f
}

// SetComment sets the value for the Comment field.
func (f *FindOptions) SetComment(comment string) *FindOptions {
	f.Comment = &comment
	return f
}

// SetCursorType sets the value for the CursorType field.
func (f *FindOptions) SetCursorType(ct CursorType) *FindOptions {
	f.CursorType = &ct
	return f
}

// SetHint sets the value for the Hint field.
func (f *FindOptions) SetHint(hint interface{}) *FindOptions {
	f.Hint = hint
	return f
}

// SetLimit sets the value for the Limit field.
func (f *FindOptions) SetLimit(i int64) *FindOptions {
	f.Limit = &i
	return f
}

// SetMax sets the value for the Max field.
func (f *FindOptions) SetMax(max interface{}) *FindOptions {
	f.Max = max
	return f
}

// SetMaxAwaitTime sets the value for the MaxAwaitTime field.
func (f *FindOptions) SetMaxAwaitTime(d time.Duration) *FindOptions {
	f.MaxAwaitTime = &d
	return f
}

// SetMaxTime specifies the max time to allow the query to run.
func (f *FindOptions) SetMaxTime(d time.Duration) *FindOptions {
	f.MaxTime = &d
	return f
}

// SetMin sets the value for the Min field.
func (f *FindOptions) SetMin(min interface{}) *FindOptions {
	f.Min = min
	return f
}

// SetNoCursorTimeout sets the value for the NoCursorTimeout field.
func (f *FindOptions) SetNoCursorTimeout(b bool) *FindOptions {
	f.NoCursorTimeout = &b
	return f
}

// SetOplogReplay sets the value for the OplogReplay field.
func (f *FindOptions) SetOplogReplay(b bool) *FindOptions {
	f.OplogReplay = &b
	return f
}

// SetProjection sets the value for the Projection field.
func (f *FindOptions) SetProjection(projection interface{}) *FindOptions {
	f.Projection = projection
	return f
}

// SetReturnKey sets the value for the ReturnKey field.
func (f *FindOptions) SetReturnKey(b bool) *FindOptions {
	f.ReturnKey = &b
	return f
}

// SetShowRecordID sets the value for the ShowRecordID field.
func (f *FindOptions) SetShowRecordID(b bool) *FindOptions {
	f.ShowRecordID = &b
	return f
}

// SetSkip sets the value for the Skip field.
func (f *FindOptions) SetSkip(i int64) *FindOptions {
	f.Skip = &i
	return f
}

// SetSnapshot sets the value for the Snapshot field.
func (f *FindOptions) SetSnapshot(b bool) *FindOptions {
	f.Snapshot = &b
	return f
}

// SetSort sets the value for the Sort field.
func (f *FindOptions) SetSort(sort interface{}) *FindOptions {
	f.Sort = sort
	return f
}

// MergeFindOptions combines the given FindOptions instances into a single FindOptions in a last-one-wins fashion.
func MergeFindOptions(opts ...*FindOptions) *FindOptions {
	fo := Find()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if opt.AllowPartialResults != nil {
			fo.AllowPartialResults = opt.AllowPartialResults
		}
		if opt.BatchSize != nil {
			fo.BatchSize = opt.BatchSize
		}
		if opt.Collation != nil {
			fo.Collation = opt.Collation
		}
		if opt.Comment != nil {
			fo.Comment = opt.Comment
		}
		if opt.CursorType != nil {
			fo.CursorType = opt.CursorType
		}
		if opt.Hint != nil {
			fo.Hint = opt.Hint
		}
		if opt.Limit != nil {
			fo.Limit = opt.Limit
		}
		if opt.Max != nil {
			fo.Max = opt.Max
		}
		if opt.MaxAwaitTime != nil {
			fo.MaxAwaitTime = opt.MaxAwaitTime
		}
		if opt.MaxTime != nil {
			fo.MaxTime = opt.MaxTime
		}
		if opt.Min != nil {
			fo.Min = opt.Min
		}
		if opt.NoCursorTimeout != nil {
			fo.NoCursorTimeout = opt.NoCursorTimeout
		}
		if opt.OplogReplay != nil {
			fo.OplogReplay = opt.OplogReplay
		}
		if opt.Projection != nil {
			fo.Projection = opt.Projection
		}
		if opt.ReturnKey != nil {
			fo.ReturnKey = opt.ReturnKey
		}
		if opt.ShowRecordID != nil {
			fo.ShowRecordID = opt.ShowRecordID
		}
		if opt.Skip != nil {
			fo.Skip = opt.Skip
		}
		if opt.Snapshot != nil {
			fo.Snapshot = opt.Snapshot
		}
		if opt.Sort != nil {
			fo.Sort = opt.Sort
		}
	}

	return fo
}

// FindOneOptions represents options that can be used to configure a FindOne operation.
type FindOneOptions struct {
	// If true, an operation on a sharded cluster can return partial results if some shards are down rather than
	// returning an error. The default value is false.
	AllowPartialResults *bool

	// The maximum number of documents to be included in each batch returned by the server.
	BatchSize *int32

	// Specifies a collation to use for string comparisons during the operation. This option is only valid for MongoDB
	// versions >= 3.4. For previous server versions, the driver will return an error if this option is used. The
	// default value is nil, which means the default collation of the collection will be used.
	Collation *Collation

	// A string that will be included in server logs, profiling logs, and currentOp queries to help trace the operation.
	// The default is the empty string, which means that no comment will be included in the logs.
	Comment *string

	// Specifies the type of cursor that should be created for the operation. The default is NonTailable, which means
	// that the cursor will be closed by the server when the last batch of documents is retrieved.
	CursorType *CursorType

	// The index to use for the aggregation. This should either be the index name as a string or the index specification
	// as a document. The default value is nil, which means that no hint will be sent.
	Hint interface{}

	// A document specifying the exclusive upper bound for a specific index. The default value is nil, which means that
	// there is no maximum value.
	Max interface{}

	// The maximum amount of time that the server should wait for new documents to satisfy a tailable cursor query.
	// This option is only valid for tailable await cursors (see the CursorType option for more information) and
	// MongoDB versions >= 3.2. For other cursor types or previous server versions, this option is ignored.
	MaxAwaitTime *time.Duration

	// The maximum amount of time that the query can run on the server. The default value is nil, meaning that there
	// is no time limit for query execution.
	MaxTime *time.Duration

	// A document specifying the inclusive lower bound for a specific index. The default value is 0, which means that
	// there is no minimum value.
	Min interface{}

	// If true, the cursor created by the operation will not timeout after a period of inactivity. The default value
	// is false.
	NoCursorTimeout *bool

	// This option is for internal replication use only and should not be set.
	OplogReplay *bool

	// A document describing which fields will be included in the document returned by the operation. The default value
	// is nil, which means all fields will be included.
	Projection interface{}

	// If true, the document returned by the operation will only contain fields corresponding to the index used. The
	// default value is false.
	ReturnKey *bool

	// If true, a $recordId field with a record identifier will be included in the document returned by the operation.
	// The default value is false.
	ShowRecordID *bool

	// The number of documents to skip before selecting the document to be returned. The default value is 0.
	Skip *int64

	// If true, the cursor will not return a document more than once because of an intervening write operation. This
	// option has been deprecated in MongoDB version 4.0. The default value is false.
	Snapshot *bool

	// A document specifying the sort order to apply to the query. The first document in the sorted order will be
	// returned.
	Sort interface{}
}

// FindOne creates a new FindOneOptions instance.
func FindOne() *FindOneOptions {
	return &FindOneOptions{}
}

// SetAllowPartialResults sets the value for the AllowPartialResults field.
func (f *FindOneOptions) SetAllowPartialResults(b bool) *FindOneOptions {
	f.AllowPartialResults = &b
	return f
}

// SetBatchSize sets the value for the BatchSize field.
func (f *FindOneOptions) SetBatchSize(i int32) *FindOneOptions {
	f.BatchSize = &i
	return f
}

// SetCollation sets the value for the Collation field.
func (f *FindOneOptions) SetCollation(collation *Collation) *FindOneOptions {
	f.Collation = collation
	return f
}

// SetComment sets the value for the Comment field.
func (f *FindOneOptions) SetComment(comment string) *FindOneOptions {
	f.Comment = &comment
	return f
}

// SetCursorType sets the value for the CursorType field.
func (f *FindOneOptions) SetCursorType(ct CursorType) *FindOneOptions {
	f.CursorType = &ct
	return f
}

// SetHint sets the value for the Hint field.
func (f *FindOneOptions) SetHint(hint interface{}) *FindOneOptions {
	f.Hint = hint
	return f
}

// SetMax sets the value for the Max field.
func (f *FindOneOptions) SetMax(max interface{}) *FindOneOptions {
	f.Max = max
	return f
}

// SetMaxAwaitTime sets the value for the MaxAwaitTime field.
func (f *FindOneOptions) SetMaxAwaitTime(d time.Duration) *FindOneOptions {
	f.MaxAwaitTime = &d
	return f
}

// SetMaxTime sets the value for the MaxTime field.
func (f *FindOneOptions) SetMaxTime(d time.Duration) *FindOneOptions {
	f.MaxTime = &d
	return f
}

// SetMin sets the value for the Min field.
func (f *FindOneOptions) SetMin(min interface{}) *FindOneOptions {
	f.Min = min
	return f
}

// SetNoCursorTimeout sets the value for the NoCursorTimeout field.
func (f *FindOneOptions) SetNoCursorTimeout(b bool) *FindOneOptions {
	f.NoCursorTimeout = &b
	return f
}

// SetOplogReplay sets the value for the OplogReplay field.
func (f *FindOneOptions) SetOplogReplay(b bool) *FindOneOptions {
	f.OplogReplay = &b
	return f
}

// SetProjection sets the value for the Projection field.
func (f *FindOneOptions) SetProjection(projection interface{}) *FindOneOptions {
	f.Projection = projection
	return f
}

// SetReturnKey sets the value for the ReturnKey field.
func (f *FindOneOptions) SetReturnKey(b bool) *FindOneOptions {
	f.ReturnKey = &b
	return f
}

// SetShowRecordID sets the value for the ShowRecordID field.
func (f *FindOneOptions) SetShowRecordID(b bool) *FindOneOptions {
	f.ShowRecordID = &b
	return f
}

// SetSkip sets the value for the Skip field.
func (f *FindOneOptions) SetSkip(i int64) *FindOneOptions {
	f.Skip = &i
	return f
}

// SetSnapshot sets the value for the Snapshot field.
func (f *FindOneOptions) SetSnapshot(b bool) *FindOneOptions {
	f.Snapshot = &b
	return f
}

// SetSort sets the value for the Sort field.
func (f *FindOneOptions) SetSort(sort interface{}) *FindOneOptions {
	f.Sort = sort
	return f
}

// MergeFindOneOptions combines the given FindOneOptions instances into a single FindOneOptions in a last-one-wins
// fashion.
func MergeFindOneOptions(opts ...*FindOneOptions) *FindOneOptions {
	fo := FindOne()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if opt.AllowPartialResults != nil {
			fo.AllowPartialResults = opt.AllowPartialResults
		}
		if opt.BatchSize != nil {
			fo.BatchSize = opt.BatchSize
		}
		if opt.Collation != nil {
			fo.Collation = opt.Collation
		}
		if opt.Comment != nil {
			fo.Comment = opt.Comment
		}
		if opt.CursorType != nil {
			fo.CursorType = opt.CursorType
		}
		if opt.Hint != nil {
			fo.Hint = opt.Hint
		}
		if opt.Max != nil {
			fo.Max = opt.Max
		}
		if opt.MaxAwaitTime != nil {
			fo.MaxAwaitTime = opt.MaxAwaitTime
		}
		if opt.MaxTime != nil {
			fo.MaxTime = opt.MaxTime
		}
		if opt.Min != nil {
			fo.Min = opt.Min
		}
		if opt.NoCursorTimeout != nil {
			fo.NoCursorTimeout = opt.NoCursorTimeout
		}
		if opt.OplogReplay != nil {
			fo.OplogReplay = opt.OplogReplay
		}
		if opt.Projection != nil {
			fo.Projection = opt.Projection
		}
		if opt.ReturnKey != nil {
			fo.ReturnKey = opt.ReturnKey
		}
		if opt.ShowRecordID != nil {
			fo.ShowRecordID = opt.ShowRecordID
		}
		if opt.Skip != nil {
			fo.Skip = opt.Skip
		}
		if opt.Snapshot != nil {
			fo.Snapshot = opt.Snapshot
		}
		if opt.Sort != nil {
			fo.Sort = opt.Sort
		}
	}

	return fo
}

// FindOneAndReplaceOptions represents options that can be used to configure a FindOneAndReplace instance.
type FindOneAndReplaceOptions struct {
	// If true, writes executed as part of the operation will opt out of document-level validation on the server. This
	// option is valid for MongoDB versions >= 3.2 and is ignored for previous server versions. The default value is
	// false. See https://docs.mongodb.com/manual/core/schema-validation/ for more information about document
	// validation.
	BypassDocumentValidation *bool

	// Specifies a collation to use for string comparisons during the operation. This option is only valid for MongoDB
	// versions >= 3.4. For previous server versions, the driver will return an error if this option is used. The
	// default value is nil, which means the default collation of the collection will be used.
	Collation *Collation

	// The maximum amount of time that the query can run on the server. The default value is nil, meaning that there
	// is no time limit for query execution.
	MaxTime *time.Duration

	// A document describing which fields will be included in the document returned by the operation. The default value
	// is nil, which means all fields will be included.
	Projection interface{}

	// Specifies whether the original or replaced document should be returned by the operation. The default value is
	// Before, which means the original document will be returned from before the replacement is performed.
	ReturnDocument *ReturnDocument

	// A document specifying which document should be replaced if the filter used by the operation matches multiple
	// documents in the collection. If set, the first document in the sorted order will be replaced.
	// The default value is nil.
	Sort interface{}

	// If true, a new document will be inserted if the filter does not match any documents in the collection. The
	// default value is false.
	Upsert *bool
}

// FindOneAndReplace creates a new FindOneAndReplaceOptions instance.
func FindOneAndReplace() *FindOneAndReplaceOptions {
	return &FindOneAndReplaceOptions{}
}

// SetBypassDocumentValidation sets the value for the BypassDocumentValidation field.
func (f *FindOneAndReplaceOptions) SetBypassDocumentValidation(b bool) *FindOneAndReplaceOptions {
	f.BypassDocumentValidation = &b
	return f
}

// SetCollation sets the value for the Collation field.
func (f *FindOneAndReplaceOptions) SetCollation(collation *Collation) *FindOneAndReplaceOptions {
	f.Collation = collation
	return f
}

// SetMaxTime sets the value for the MaxTime field.
func (f *FindOneAndReplaceOptions) SetMaxTime(d time.Duration) *FindOneAndReplaceOptions {
	f.MaxTime = &d
	return f
}

// SetProjection sets the value for the Projection field.
func (f *FindOneAndReplaceOptions) SetProjection(projection interface{}) *FindOneAndReplaceOptions {
	f.Projection = projection
	return f
}

// SetReturnDocument sets the value for the ReturnDocument field.
func (f *FindOneAndReplaceOptions) SetReturnDocument(rd ReturnDocument) *FindOneAndReplaceOptions {
	f.ReturnDocument = &rd
	return f
}

// SetSort sets the value for the Sort field.
func (f *FindOneAndReplaceOptions) SetSort(sort interface{}) *FindOneAndReplaceOptions {
	f.Sort = sort
	return f
}

// SetUpsert sets the value for the Upsert field.
func (f *FindOneAndReplaceOptions) SetUpsert(b bool) *FindOneAndReplaceOptions {
	f.Upsert = &b
	return f
}

// MergeFindOneAndReplaceOptions combines the given FindOneAndReplaceOptions instances into a single
// FindOneAndReplaceOptions in a last-one-wins fashion.
func MergeFindOneAndReplaceOptions(opts ...*FindOneAndReplaceOptions) *FindOneAndReplaceOptions {
	fo := FindOneAndReplace()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if opt.BypassDocumentValidation != nil {
			fo.BypassDocumentValidation = opt.BypassDocumentValidation
		}
		if opt.Collation != nil {
			fo.Collation = opt.Collation
		}
		if opt.MaxTime != nil {
			fo.MaxTime = opt.MaxTime
		}
		if opt.Projection != nil {
			fo.Projection = opt.Projection
		}
		if opt.ReturnDocument != nil {
			fo.ReturnDocument = opt.ReturnDocument
		}
		if opt.Sort != nil {
			fo.Sort = opt.Sort
		}
		if opt.Upsert != nil {
			fo.Upsert = opt.Upsert
		}
	}

	return fo
}

// FindOneAndUpdateOptions represents options that can be used to configure a FindOneAndUpdate options.
type FindOneAndUpdateOptions struct {
	// A set of filters specifying to which array elements an update should apply. This option is only valid for MongoDB
	// versions >= 3.6. For previous server versions, the driver will return an error if this option is used. The
	// default value is nil, which means the update will apply to all array elements.
	ArrayFilters *ArrayFilters

	// If true, writes executed as part of the operation will opt out of document-level validation on the server. This
	// option is valid for MongoDB versions >= 3.2 and is ignored for previous server versions. The default value is
	// false. See https://docs.mongodb.com/manual/core/schema-validation/ for more information about document
	// validation.
	BypassDocumentValidation *bool

	// Specifies a collation to use for string comparisons during the operation. This option is only valid for MongoDB
	// versions >= 3.4. For previous server versions, the driver will return an error if this option is used. The
	// default value is nil, which means the default collation of the collection will be used.
	Collation *Collation

	// The maximum amount of time that the query can run on the server. The default value is nil, meaning that there
	// is no time limit for query execution.
	MaxTime *time.Duration

	// A document describing which fields will be included in the document returned by the operation. The default value
	// is nil, which means all fields will be included.
	Projection interface{}

	// Specifies whether the original or replaced document should be returned by the operation. The default value is
	// Before, which means the original document will be returned before the replacement is performed.
	ReturnDocument *ReturnDocument

	// A document specifying which document should be updated if the filter used by the operation matches multiple
	// documents in the collection. If set, the first document in the sorted order will be updated.
	// The default value is nil.
	Sort interface{}

	// If true, a new document will be inserted if the filter does not match any documents in the collection. The
	// default value is false.
	Upsert *bool
}

// FindOneAndUpdate creates a new FindOneAndUpdateOptions instance.
func FindOneAndUpdate() *FindOneAndUpdateOptions {
	return &FindOneAndUpdateOptions{}
}

// SetBypassDocumentValidation sets the value for the BypassDocumentValidation field.
func (f *FindOneAndUpdateOptions) SetBypassDocumentValidation(b bool) *FindOneAndUpdateOptions {
	f.BypassDocumentValidation = &b
	return f
}

// SetArrayFilters sets the value for the ArrayFilters field.
func (f *FindOneAndUpdateOptions) SetArrayFilters(filters ArrayFilters) *FindOneAndUpdateOptions {
	f.ArrayFilters = &filters
	return f
}

// SetCollation sets the value for the Collation field.
func (f *FindOneAndUpdateOptions) SetCollation(collation *Collation) *FindOneAndUpdateOptions {
	f.Collation = collation
	return f
}

// SetMaxTime sets the value for the MaxTime field.
func (f *FindOneAndUpdateOptions) SetMaxTime(d time.Duration) *FindOneAndUpdateOptions {
	f.MaxTime = &d
	return f
}

// SetProjection sets the value for the Projection field.
func (f *FindOneAndUpdateOptions) SetProjection(projection interface{}) *FindOneAndUpdateOptions {
	f.Projection = projection
	return f
}

// SetReturnDocument sets the value for the ReturnDocument field.
func (f *FindOneAndUpdateOptions) SetReturnDocument(rd ReturnDocument) *FindOneAndUpdateOptions {
	f.ReturnDocument = &rd
	return f
}

// SetSort sets the value for the Sort field.
func (f *FindOneAndUpdateOptions) SetSort(sort interface{}) *FindOneAndUpdateOptions {
	f.Sort = sort
	return f
}

// SetUpsert sets the value for the Upsert field.
func (f *FindOneAndUpdateOptions) SetUpsert(b bool) *FindOneAndUpdateOptions {
	f.Upsert = &b
	return f
}

// MergeFindOneAndUpdateOptions combines the given FindOneAndUpdateOptions instances into a single
// FindOneAndUpdateOptions in a last-one-wins fashion.
func MergeFindOneAndUpdateOptions(opts ...*FindOneAndUpdateOptions) *FindOneAndUpdateOptions {
	fo := FindOneAndUpdate()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if opt.ArrayFilters != nil {
			fo.ArrayFilters = opt.ArrayFilters
		}
		if opt.BypassDocumentValidation != nil {
			fo.BypassDocumentValidation = opt.BypassDocumentValidation
		}
		if opt.Collation != nil {
			fo.Collation = opt.Collation
		}
		if opt.MaxTime != nil {
			fo.MaxTime = opt.MaxTime
		}
		if opt.Projection != nil {
			fo.Projection = opt.Projection
		}
		if opt.ReturnDocument != nil {
			fo.ReturnDocument = opt.ReturnDocument
		}
		if opt.Sort != nil {
			fo.Sort = opt.Sort
		}
		if opt.Upsert != nil {
			fo.Upsert = opt.Upsert
		}
	}

	return fo
}

// FindOneAndDeleteOptions represents options that can be used to configure a FindOneAndDelete operation.
type FindOneAndDeleteOptions struct {
	// Specifies a collation to use for string comparisons during the operation. This option is only valid for MongoDB
	// versions >= 3.4. For previous server versions, the driver will return an error if this option is used. The
	// default value is nil, which means the default collation of the collection will be used.
	Collation *Collation

	// The maximum amount of time that the query can run on the server. The default value is nil, meaning that there
	// is no time limit for query execution.
	MaxTime *time.Duration

	// A document describing which fields will be included in the document returned by the operation. The default value
	// is nil, which means all fields will be included.
	Projection interface{}

	// A document specifying which document should be replaced if the filter used by the operation matches multiple
	// documents in the collection. If set, the first document in the sorted order will be selected for replacement.
	// The default value is nil.
	Sort interface{}
}

// FindOneAndDelete creates a new FindOneAndDeleteOptions instance.
func FindOneAndDelete() *FindOneAndDeleteOptions {
	return &FindOneAndDeleteOptions{}
}

// SetCollation sets the value for the Collation field.
func (f *FindOneAndDeleteOptions) SetCollation(collation *Collation) *FindOneAndDeleteOptions {
	f.Collation = collation
	return f
}

// SetMaxTime sets the value for the MaxTime field.
func (f *FindOneAndDeleteOptions) SetMaxTime(d time.Duration) *FindOneAndDeleteOptions {
	f.MaxTime = &d
	return f
}

// SetProjection sets the value for the Projection field.
func (f *FindOneAndDeleteOptions) SetProjection(projection interface{}) *FindOneAndDeleteOptions {
	f.Projection = projection
	return f
}

// SetSort sets the value for the Sort field.
func (f *FindOneAndDeleteOptions) SetSort(sort interface{}) *FindOneAndDeleteOptions {
	f.Sort = sort
	return f
}

// MergeFindOneAndDeleteOptions combines the given FindOneAndDeleteOptions instances into a single
// FindOneAndDeleteOptions in a last-one-wins fashion.
func MergeFindOneAndDeleteOptions(opts ...*FindOneAndDeleteOptions) *FindOneAndDeleteOptions {
	fo := FindOneAndDelete()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if opt.Collation != nil {
			fo.Collation = opt.Collation
		}
		if opt.MaxTime != nil {
			fo.MaxTime = opt.MaxTime
		}
		if opt.Projection != nil {
			fo.Projection = opt.Projection
		}
		if opt.Sort != nil {
			fo.Sort = opt.Sort
		}
	}

	return fo
}
