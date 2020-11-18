#!/usr/bin/perl
# no Preamble to keep it light
use Sys::Syslog qw(:standard :macros);

openlog($ARGV[0], "pid", "daemon");
my %lvl_map = (
    'panic' => LOG_EMERG,
    'fatal' => LOG_CRIT,
    'error' => LOG_ERR,
    'warn'  => LOG_WARNING,
    'info'  => LOG_INFO,
);

while (my $l = <STDIN>) {
  my ($lvl, undef, $message) = split /\t/, $_, 3;
  next unless $message;
  $lvl = $lvl_map{ lc $lvl } || 'warn';
  chomp $message;
  syslog( $lvl, $message );
}

closelog();
