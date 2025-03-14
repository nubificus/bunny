`bunny` is an open-source project licenced under the [Apache License
2.0](https://github.com/nubificus/urunc/blob/main/LICENSE).  We welcome anyone
who would be interested in contributing to `bunny`.  As a first step, please
take a look at the following document.  The current document provides a high
level overview of `bunny`'s code structure, along with a few guidelines
regarding contributions to the project.

## Table of contents:

1. [Code organization](#code-organization)
2. [How to contribute](#how-to-contribute)
3. [Opening an issue](#opening-an-issue)
4. [Requesting new features](#requesting-new-features)
5. [Submitting a PR](#submitting-a-pr)
6. [Style guide](#style-guide)

## Code organization

`bunny` is written in Go and we structure the code and other files as follows:

- `/`: The root directory contains the Makefile to build `bunny`, along with
  other non-code files, such as the licence, Readme and more.
- `/docs`: This directory contains all the documentation related to `bunny`.
- `/examples`: This directory contains various examples to help users get familiar with `bunny`.
- `/cmd`: This directory contains handlers for the various command line options
  of `bunny`.
- `/hops`: This directory contains the majority of the code for `bunny` and in
  particular the part of code that transforms a `bunnyfile` to a LLB.

Therefore, we expect any new documentation related files to be placed under
`/docs` and any changes or new files in code to be either in the `/cmd/` or
`/pkg/` directory. Furthermore, we try to provide an example for each feature
of `bunny` and hence all examples should be under the `/examples` directory.

## How to contribute

There are plenty of ways to contribute to an open source project, even without
changing or touching the code. Therefore, anyone who is interested in this
project is very welcome to contribute in one of the following ways:

1.  Using `bunny`. Try it out yourself and let us know your experience. Did
    everything work well? Were the instructions clear?
2.  Improve or suggest changes to the documentation of the project.
    Documentation is very important for every project, hence any ideas on how
    to improve the documentation to make it more clear are more than welcome.
3.  Request new features. Any proposals for improving or adding new features
    are very welcome.
4.  Find a bug and report it. Bugs are everywhere and some are hidden very
    well. As a result, we would really appreciate it if someone found a bug and
    reported it to the maintainers.
5.  Make changes to the code. Improve the code, add new functionalities and
    make `bunny` even more useful.

## Opening an issue

We use Github issues to track bugs and requests for new features.  Anyone is
welcome to open a new issue, which is either related to a bug or a request for
a new feature.

### Reporting bugs

In order to report a bug or misbehavior in `bunny`, a user can open a new issue
explaining the problem. For the time being, we do not use any strict template
for reporting any issues. However, in order to easily identify and fix the
problem, it would be very helpful to provide enough information. In that
context, when opening a new issue regarding a bug, we kindly ask you to:

- Mark the issue with the bug label
- Provide the following information:

    1. A short description of the bug.
    2. The respective logs both from the output and containerd.
    3. `bunny`'s version (either the commit's hash or the version).
    4. The CPU architecture, VMM and the Unikernel framework used.
    5. Any particular steps to reproduce the issue.
- Keep an eye on the issue for possible questions from the maintainers.

A template for an issue could be the following one:
```
## Description
An explanation of the issue 

## System info

- `bunny` version:
- Arch:
- VMM:
- Unikernel:

## Steps to reproduce
A list of steps that can reproduce the issue.
```

### Requesting new features

We will be very happy to listen from users about new features that they would
like to see in `bunny`. One way to communicate such a request is using Github
issues. For the time being, we do not use any strict template for requesting
new features. However, we kindly ask you to mark the issue with the
enhancement label and provide a description of the new feature.

## Submitting a PR

Anyone should feel free to submit a change or an addition to the codebase of `bunny`.
Currently, we use Github's Pull Requests (PRs) to submit changes to `bunny`'s codebase.
Before creating a new PR, please follow the guidelines below:

- Make sure that the changes do not break the building process of `bunny`.
- Make sure that all the tests run successfully.
- Make sure that no commit in a PR breaks the building process of `bunny`
- Make sure to sign-off your commits.
- Provide meaningful commit messages, describing shortly the changes.
- Provide a meaningful PR message

As soon as a new PR is created the following workflow will take place:

  1. When a PR opens the tests will start running
  2. If the tests pass, request from one or more `bunny`'s maintainers to review the PR.
  3. The reviewers submit their review.
  4. The author of the PR should address all the comments from the reviewers.
  5. As soon as a reviewer approves the PR, an action will add the appropriate git trailers in the PR's commits.
  6. The reviewer who accepted the changes will merge the new changes.

## Style guide

### Git commit messages

Please follow the below guidelines for your commit messages:

- Limit the first line to 72 characters or less.
- Limit all the other lines to 80 characters
- In case the PR is associated with an issue, please refer to it, using the git trailer `Fixes: #Nr_issue`
- Always sign-off your commit message

### Golang code styde

We follow gofmt's rules on formatting GO code. Therefore, we ask all
contributors to do the same.  Go provides the `gofmt` tool, which can be used
for formatting your code.
