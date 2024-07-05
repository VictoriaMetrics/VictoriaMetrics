# These commands must be run from the VictoriaMetrics repository root.

# Builds cspell image.
cspell-build:
	@docker build cspell \
		--tag cspell

# Checks for spelling errors.
cspell-check:
	@CMD="cspell --no-progress" $(MAKE) cspell-run-command

# Runs cspell container commands.
cspell-run-command:
	@docker run \
		--entrypoint /bin/sh \
		--mount type=bind,src=".",dst=/victoriametrics \
		--rm \
		--tty \
		cspell -c "$(CMD)"

# Updates cspell dictionaries and other NPM dependencies.
cspell-update: cspell-build
	@CMD="cd /victoriametrics/cspell && npm update && rm -rf ./node_modules" $(MAKE) cspell-run-command

# Runs spelling error check. 
# A user facing alias to cspell-check command.
spellcheck: cspell-check

# Installs spellcheck command. Needs to be run once to enable spellcheck command.
# A user facing alias to cspell-build command.
spellcheck-install: cspell-build
	@echo
	@echo "'make spellcheck' command is ready."
