# Contributing

* Install the follow packages: ``git``, ``kubectl``, ``helm``, ``helm-docs``. See this [tutorial](REQUIREMENTS.md).
* [OPTIONAL] Configure authentication on your Github account to use the SSH protocol instead of HTTP. Watch this tutorial to learn how to set up: https://help.github.com/en/github/authenticating-to-github/adding-a-new-ssh-key-to-your-github-account
* Create a fork this repository.
* Clone the forked repository to your local system:

```bash
git clone URL_FORKED_REPOSITORY
```

* Add the address for the remote original repository:

```bash
git remote -v
git remote add upstream https://github.com/VictoriaMetrics/helm-charts
git remote -v
```

* Create a branch. Example:

```bash
git checkout -b BRANCH_NAME
```

* Make sure you are on the correct branch using the following command. The branch in use contains the '*' before the name.

```bash
git branch
```

* Make your changes and tests to the new branch.
* Run command ``helm-docs`` to update content of ``README.md`` file of all charts using the ``README.md.gotmpl`` template.
* Commit the changes to the branch.
* Push files to repository remote with command:

```bash
git push --set-upstream origin BRANCH_NAME
```

* Create Pull Request (PR) to the `master` branch. See this [tutorial](https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/creating-a-pull-request-from-a-fork)
* Update the content with the suggestions of the reviewer (if necessary).
* After your pull request is merged to the `master` branch, update your local clone:

```bash
git checkout master
git pull upstream master
```

* Clean up after your pull request is merged with command:

```bash
git branch -d BRANCH_NAME
```

* Then you can update the ``master`` branch in your forked repository.

```bash
git push origin master
```

* And push the deletion of the feature branch to your GitHub repository with command:

```bash
git push --delete origin BRANCH_NAME
```

* To keep your fork in sync with the original repository, use these commands:

```bash
git pull upstream master
git push origin master
```

Reference:
* https://blog.scottlowe.org/2015/01/27/using-fork-branch-git-workflow/