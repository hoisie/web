task :default => :server

desc 'Build site with Jekyll'
task :build do
  jekyll
end

desc 'Build and start server with --auto'
task :server do
  jekyll 'serve --safe --watch'
end

def jekyll(opts = '')
  sh 'rm -rf _site'
  sh 'bundle exec jekyll ' + opts
end
