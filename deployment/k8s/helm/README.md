### Victoria metrics helm chart

#### Create cluster from chart

```$bash
$ ENV=<env> make helm-install
```

for DEV env :

```$bash
$ make helm-install-dev
```

#### Upgrade cluster from chart

```$bash
$ ENV=<env> make helm-upgrade
```

for DEV env :

```$bash
$ make helm-upgrade-dev
```

#### Delete chart from cluster

```$bash
$ ENV=<env> make helm-delete
```

for DEV env :

```$bash
$ make helm-delete-dev
```
