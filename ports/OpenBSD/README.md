# OpenBSD ports

Tested with Release 6.9

The VictoriaMetrics DB must be place in `/usr/ports/databases` directory
and the file `/usr/ports/infrastructure/db/user.list` should be modified
with a new line

```lang-none
866 _vmetrics            _vmetrics       databases/VictoriaMetrics
```
