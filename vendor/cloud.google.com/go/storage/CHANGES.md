# Changes

### [1.16.1](https://www.github.com/googleapis/google-cloud-go/compare/storage/v1.16.0...storage/v1.16.1) (2021-08-30)


### Bug Fixes

* **storage/internal:** Update encryption_key fields to "bytes" type. fix: Improve date/times and field name clarity in lifecycle conditions. ([a52baa4](https://www.github.com/googleapis/google-cloud-go/commit/a52baa456ed8513ec492c4b573c191eb61468758))
* **storage:** accept emulator env var without scheme ([#4616](https://www.github.com/googleapis/google-cloud-go/issues/4616)) ([5f8cbb9](https://www.github.com/googleapis/google-cloud-go/commit/5f8cbb98070109e2a34409ac775ed63b94d37efd))
* **storage:** preserve supplied endpoint's scheme ([#4609](https://www.github.com/googleapis/google-cloud-go/issues/4609)) ([ee2756f](https://www.github.com/googleapis/google-cloud-go/commit/ee2756fb0a335d591464a770c9fa4f8fe0ba2e01))
* **storage:** remove unnecessary variable ([#4608](https://www.github.com/googleapis/google-cloud-go/issues/4608)) ([27fc784](https://www.github.com/googleapis/google-cloud-go/commit/27fc78456fb251652bdf5cdb493734a7e1e643e1))
* **storage:** retry LockRetentionPolicy ([#4439](https://www.github.com/googleapis/google-cloud-go/issues/4439)) ([09879ea](https://www.github.com/googleapis/google-cloud-go/commit/09879ea80cb67f9bfd8fc9384b0fda335567cba9)), refs [#4437](https://www.github.com/googleapis/google-cloud-go/issues/4437)
* **storage:** revise Reader to send XML preconditions ([#4479](https://www.github.com/googleapis/google-cloud-go/issues/4479)) ([e36b29a](https://www.github.com/googleapis/google-cloud-go/commit/e36b29a3d43bce5c1c044f7daf6e1db00b0a49e0)), refs [#4470](https://www.github.com/googleapis/google-cloud-go/issues/4470)

## [1.16.0](https://www.github.com/googleapis/google-cloud-go/compare/storage/v1.15.0...storage/v1.16.0) (2021-06-28)


### Features

* **storage:** support PublicAccessPrevention ([#3608](https://www.github.com/googleapis/google-cloud-go/issues/3608)) ([99bc782](https://www.github.com/googleapis/google-cloud-go/commit/99bc782fb50a47602b45278384ef5d5b5da9263b)), refs [#3203](https://www.github.com/googleapis/google-cloud-go/issues/3203)


### Bug Fixes

* **storage:** fix Writer.ChunkSize validation ([#4255](https://www.github.com/googleapis/google-cloud-go/issues/4255)) ([69c2e9d](https://www.github.com/googleapis/google-cloud-go/commit/69c2e9dc6303e1a004d3104a8178532fa738e742)), refs [#4167](https://www.github.com/googleapis/google-cloud-go/issues/4167)
* **storage:** try to reopen for failed Reads ([#4226](https://www.github.com/googleapis/google-cloud-go/issues/4226)) ([564102b](https://www.github.com/googleapis/google-cloud-go/commit/564102b335dbfb558bec8af883e5f898efb5dd10)), refs [#3040](https://www.github.com/googleapis/google-cloud-go/issues/3040)

## [1.15.0](https://www.github.com/googleapis/google-cloud-go/compare/storage/v1.13.0...storage/v1.15.0) (2021-04-21)


### Features

* **transport** Bump dependency on google.golang.org/api to pick up HTTP/2
  config updates (see [googleapis/google-api-go-client#882](https://github.com/googleapis/google-api-go-client/pull/882)).

### Bug Fixes

* **storage:** retry io.ErrUnexpectedEOF ([#3957](https://www.github.com/googleapis/google-cloud-go/issues/3957)) ([f6590cd](https://www.github.com/googleapis/google-cloud-go/commit/f6590cdc26c8479be5df48949fa59f879e0c24fc))


## v1.14.0

- Updates to various dependencies.

## [1.13.0](https://www.github.com/googleapis/google-cloud-go/compare/storage/v1.12.0...v1.13.0) (2021-02-03)


### Features

* **storage:** add missing StorageClass in BucketAttrsToUpdate ([#3038](https://www.github.com/googleapis/google-cloud-go/issues/3038)) ([2fa1b72](https://www.github.com/googleapis/google-cloud-go/commit/2fa1b727f8a7b20aa62fe0990530744f6c109be0))
* **storage:** add projection parameter for BucketHandle.Objects() ([#3549](https://www.github.com/googleapis/google-cloud-go/issues/3549)) ([9b9c3dc](https://www.github.com/googleapis/google-cloud-go/commit/9b9c3dce3ee10af5b6c4d070821bf47a861efd5b))


### Bug Fixes

* **storage:** fix endpoint selection logic ([#3172](https://www.github.com/googleapis/google-cloud-go/issues/3172)) ([99edf0d](https://www.github.com/googleapis/google-cloud-go/commit/99edf0d211a9e617f2586fbc83b6f9630da3c537))

## v1.12.0
- V4 signed URL fixes:
  - Fix encoding of spaces in query parameters.
  - Add fields that were missing from PostPolicyV4 policy conditions.
- Fix Query to correctly list prefixes as well as objects when SetAttrSelection
  is used.

## v1.11.0
- Add support for CustomTime and NoncurrentTime object lifecycle management
  features.

## v1.10.0
- Bump dependency on google.golang.org/api to capture changes to retry logic
  which will make retries on writes more resilient.
- Improve documentation for Writer.ChunkSize.
- Fix a bug in lifecycle to allow callers to clear lifecycle rules on a bucket.

## v1.9.0
- Add retry for transient network errors on most operations (with the exception
  of writes).
- Bump dependency for google.golang.org/api to capture a change in the default
  HTTP transport which will improve performance for reads under heavy load.
- Add CRC32C checksum validation option to Composer.

## v1.8.0
- Add support for V4 signed post policies.

## v1.7.0
- V4 signed URL support:
  - Add support for bucket-bound domains and virtual hosted style URLs.
  - Add support for query parameters in the signature.
  - Fix text encoding to align with standards.
- Add the object name to query parameters for write calls.
- Fix retry behavior when reading files with Content-Encoding gzip.
- Fix response header in reader.
- New code examples:
   - Error handling for `ObjectHandle` preconditions.
   - Existence checks for buckets and objects.

## v1.6.0

- Updated option handling:
  - Don't drop custom scopes (#1756)
  - Don't drop port in provided endpoint (#1737)

## v1.5.0

- Honor WithEndpoint client option for reads as well as writes.
- Add archive storage class to docs.
- Make fixes to storage benchwrapper.

## v1.4.0

- When listing objects in a bucket, allow callers to specify which attributes
  are queried. This allows for performance optimization.

## v1.3.0

- Use `storage.googleapis.com/storage/v1` by default for GCS requests
  instead of `www.googleapis.com/storage/v1`.

## v1.2.1

- Fixed a bug where UniformBucketLevelAccess and BucketPolicyOnly were not
  being sent in all cases.

## v1.2.0

- Add support for UniformBucketLevelAccess. This configures access checks
  to use only bucket-level IAM policies.
  See: https://godoc.org/cloud.google.com/go/storage#UniformBucketLevelAccess.
- Fix userAgent to use correct version.

## v1.1.2

- Fix memory leak in BucketIterator and ObjectIterator.

## v1.1.1

- Send BucketPolicyOnly even when it's disabled.

## v1.1.0

- Performance improvements for ObjectIterator and BucketIterator.
- Fix Bucket.ObjectIterator size calculation checks.
- Added HMACKeyOptions to all the methods which allows for options such as
  UserProject to be set per invocation and optionally be used.

## v1.0.0

This is the first tag to carve out storage as its own module. See:
https://github.com/golang/go/wiki/Modules#is-it-possible-to-add-a-module-to-a-multi-module-repository.
