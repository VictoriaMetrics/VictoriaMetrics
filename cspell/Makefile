# These commands must be run from the VictoriaMetrics repository root.

# Builds cspell image.
cspell-install:
	@ (docker inspect cspell > /dev/null) || (docker build cspell --tag cspell)

# Checks for spelling errors.
cspell-check: cspell-install
	@CMD="cspell --no-progress" $(MAKE) cspell-run-command

# Runs spelling error check.
# A user facing alias to cspell-check command.
spellcheck: cspell-check

# Runs cspell container commands.
cspell-run-command:
	@cp cspell/cspell.json cspell.json
	@-docker run \
		--entrypoint /bin/sh \
		--mount type=bind,src=".",dst=/victoriametrics \
		--rm \
		--tty \
		cspell -c "$(CMD)"
	@rm cspell.json

cspell-update-deps: cspell-install
	@CMD="cd /victoriametrics/cspell && npm update && rm -rf ./node_modules" $(MAKE) cspell-run-command

