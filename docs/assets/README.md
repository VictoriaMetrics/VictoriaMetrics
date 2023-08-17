This folder must contain only files, which are needed for generating https://docs.victoriametrics.com .

This folder **should not contain** files specific for a particular documentation pages such as images
used in a particular doc. Such files must be placed in the same folder as the doc itself
and they must have the same prefix as the doc filename. For example, all the images for docs/foo/bar.md
should have filenames starting from docs/foo/bar. This simplifies lifetime management for these files.
For example, if the corresponding doc is removed, then it is easy to remove all the associated
images with a simple `rm -rf docs/foo/bar*` command. This also simplifies referring the associated images
from docs displayed at various views:

- https://docs.victoriametrics.com
- https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/docs
