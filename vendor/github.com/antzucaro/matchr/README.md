# matchr

An approximate string matching library for the [Go programming language](http://www.golang.org).

## Rationale

Data used in record linkage can often be of dubious quality. Typographical 
errors or changing data elements (to name a few things) make establishing similarity between two sets of data 
difficult. Rather than use exact string comparison in such situations, it is
vital to have a means to identify how similar two strings are. Similarity functions can cater
to certain data sets in order to make better matching decisions. The matchr library provides
several of these similarity functions.
