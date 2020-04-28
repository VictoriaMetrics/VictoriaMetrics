// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package options

// DeleteOptions represents options that can be used to configure DeleteOne and DeleteMany operations.
type DeleteOptions struct {
	// Specifies a collation to use for string comparisons during the operation. This option is only valid for MongoDB
	// versions >= 3.4. For previous server versions, the driver will return an error if this option is used. The
	// default value is nil, which means the default collation of the collection will be used.
	Collation *Collation
}

// Delete creates a new DeleteOptions instance.
func Delete() *DeleteOptions {
	return &DeleteOptions{}
}

// SetCollation sets the value for the Collation field.
func (do *DeleteOptions) SetCollation(c *Collation) *DeleteOptions {
	do.Collation = c
	return do
}

// MergeDeleteOptions combines the given DeleteOptions instances into a single DeleteOptions in a last-one-wins fashion.
func MergeDeleteOptions(opts ...*DeleteOptions) *DeleteOptions {
	dOpts := Delete()
	for _, do := range opts {
		if do == nil {
			continue
		}
		if do.Collation != nil {
			dOpts.Collation = do.Collation
		}
	}

	return dOpts
}
