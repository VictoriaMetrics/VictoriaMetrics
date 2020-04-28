// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package options

// UpdateOptions represents options that can be used to configure UpdateOne and UpdateMany operations.
type UpdateOptions struct {
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

	// If true, a new document will be inserted if the filter does not match any documents in the collection. The
	// default value is false.
	Upsert *bool
}

// Update creates a new UpdateOptions instance.
func Update() *UpdateOptions {
	return &UpdateOptions{}
}

// SetArrayFilters sets the value for the ArrayFilters field.
func (uo *UpdateOptions) SetArrayFilters(af ArrayFilters) *UpdateOptions {
	uo.ArrayFilters = &af
	return uo
}

// SetBypassDocumentValidation sets the value for the BypassDocumentValidation field.
func (uo *UpdateOptions) SetBypassDocumentValidation(b bool) *UpdateOptions {
	uo.BypassDocumentValidation = &b
	return uo
}

// SetCollation sets the value for the Collation field.
func (uo *UpdateOptions) SetCollation(c *Collation) *UpdateOptions {
	uo.Collation = c
	return uo
}

// SetUpsert sets the value for the Upsert field.
func (uo *UpdateOptions) SetUpsert(b bool) *UpdateOptions {
	uo.Upsert = &b
	return uo
}

// MergeUpdateOptions combines the given UpdateOptions instances into a single UpdateOptions in a last-one-wins fashion.
func MergeUpdateOptions(opts ...*UpdateOptions) *UpdateOptions {
	uOpts := Update()
	for _, uo := range opts {
		if uo == nil {
			continue
		}
		if uo.ArrayFilters != nil {
			uOpts.ArrayFilters = uo.ArrayFilters
		}
		if uo.BypassDocumentValidation != nil {
			uOpts.BypassDocumentValidation = uo.BypassDocumentValidation
		}
		if uo.Collation != nil {
			uOpts.Collation = uo.Collation
		}
		if uo.Upsert != nil {
			uOpts.Upsert = uo.Upsert
		}
	}

	return uOpts
}
