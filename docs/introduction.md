<div class="logo" aria-hidden="true">
{{#include theme/mark.svg}}
</div>

# umbra

umbra is named for the three regions of a shadow, and sorts every column of your
database into one of them:

| Region | Class | What a view shows |
|---|---|---|
| lit | `allow` | the value, as it is |
| penumbra | a masker, such as `generalise=domain` | a value derived from it: a domain, a pseudonym |
| umbra | `deny=<reason>` | nothing |

The class is written once, next to the model that stores the column, and from there
umbra does three things: it keeps the classes as small files that reviewers read, it
checks them against the schema the database really has, and it renders the views.
The check is what makes the views safe: a column that nobody classified is an error,
not a column that quietly shows up.

<figure class="diagram">
{{#include diagrams/how-it-works.svg}}
<figcaption>Classes are written once, kept as files, checked against the database, and rendered as views.</figcaption>
</figure>

## Two ways in

- **In Go**, as a library with nothing but the standard library behind it. You tag
  your models, register them, and call umbra from a test and from the code that
  writes your migrations. [Getting started](getting-started.md) lays that out in a
  project, and the README has the shortest example.
- **Without Go**, as the `umbra` command. The catalog is files and a small
  `umbra.toml`, which the command checks in CI and renders into your migrations. See
  the [command line](command-line.md).

The API reference, with a runnable example for each step, is on
[pkg.go.dev](https://pkg.go.dev/github.com/feds01/umbra).
