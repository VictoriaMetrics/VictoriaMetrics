# These commands must be run from the VictoriaMetrics repository root.

# Builds cspell image.
cspell-install:
	@docker build cspell -t cspell

# Checks for spelling errors.
cspell-check: CMD="--no-progress -r /vm"
cspell-check: cspell-install cspell-run

# Runs spelling error check.
# A user facing alias to cspell-check command.
spellcheck: cspell-check

# Runs cspell container commands.
cspell-run:
	@-docker run \
		--mount type=bind,src="$(PWD)",dst=/vm \
		--rm \
		cspell -c cspell/cspell.json "$(CMD)"

cspell-update-deps: cspell-install
	@-docker run \
		--mount type=bind,src="$(PWD)",dst=/vm \
		--entrypoint=/bin/sh \
		--workdir=/vm/cspell \
		--rm \
		cspell -c "yarn install && yarn upgrade && rm -rf ./node_modules"
