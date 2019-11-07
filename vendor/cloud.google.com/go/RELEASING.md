# Expectations

In CONTRIBUTING.md, we expect users to have a remote called `origin` that points
to the `https://code.googlesource.com/gocloud` repository.  For releasing, we
also use a `github` remote, setup to point to the replica repository hosted
on github at `https://github.com/googleapis/google-cloud-go`

If you don't already have an 'origin' remote from cloning the master repository,
you can add it:

`git remote add origin https://code.googlesource.com/gocloud`

Add the github remote as well, using the name `github`:

`git remote add github https://github.com/googleapis/google-cloud-go`

# How to release `cloud.google.com/go`

1. Navigate `google-cloud-go/` and switch to master.
1. `git pull`
1. Determine the current release version with `git tag -l`. It should look
   something like `vX.Y.Z`. We'll call the current version `$CV` and the new
   version `$NV`.
1. On master, run `git log $CV...` to list all the changes since the last
   release. NOTE: You must manually exclude changes from submodules [1].
1. Edit `CHANGES.md` to include a summary of the changes.
1. `cd internal/version && go generate && cd -`
1. `./tidyall.sh`
1. Mail the CL: `git add -A && git change && git mail`
1. Wait for the CL to be submitted. Once it's submitted, and without submitting
   any other CLs in the meantime:
   a. Switch to master.
   b. `git pull`
   c. Tag the repo with the next version: `git tag $NV`.
   d. Push the tag to both the googlesource and github repositories:
      `git push origin $NV`
      `git push github $NV`
1. Update [the releases page](https://github.com/googleapis/google-cloud-go/releases)
   with the new release, copying the contents of `CHANGES.md`.

# How to release a submodule

We have several submodules, including cloud.google.com/go/logging,
cloud.google.com/go/datastore, and so on.

To release a submodule:

(these instructions assume we're releasing cloud.google.com/go/datastore - adjust accordingly)

1. Navigate `google-cloud-go/` and switch to master.
1. `git pull`
1. Determine the current release version with `git tag -l | grep datastore`. It
   should look something like `datastore/vX.Y.Z`. We'll call the current version
   `$CV` and the new version `$NV`, which should look something like `datastore/vX.Y+1.Z`
   (assuming a minor bump).
1. On master, run `git log $CV.. -- datastore/` to list all the changes to the
   submodule directory since the last release.
1. Edit `datastore/CHANGES.md` to include a summary of the changes.
1. `./tidyall.sh`
1. `cd internal/version && go generate && cd -`
1. Mail the CL: `git add -A && git change && git mail`
1. Wait for the CL to be submitted. Once it's submitted, and without submitting
   any other CLs in the meantime:
   a. Switch to master.
   b. `git pull`
   c. Tag the repo with the next version: `git tag $NV`.
   d. Push the tag to both the googlesource and github repositories:
      `git push origin $NV`
      `git push github $NV`
1. Update [the releases page](https://github.com/googleapis/google-cloud-go/releases)
   with the new release, copying the contents of `datastore/CHANGES.md`.

# Appendix

1: This should get better as submodule tooling matures.
