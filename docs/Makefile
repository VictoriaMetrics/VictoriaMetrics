docs-install:
	gem install jekyll bundler
	bundle install --gemfile=Gemfile

# To run localy you need to use ruby version =< 2.7.6, but not >=3.x , see https://bbs.archlinux.org/viewtopic.php?pid=1976408#p1976408
#
# run local server for documentation website
# at http://127.0.0.1:4000/
# On first use, please run `make docs-install`
docs-up:
	JEKYLL_GITHUB_TOKEN=blank PAGES_API_URL=http://0.0.0.0 bundle exec \
		--gemfile=Gemfile \
	 	jekyll server --livereload
