#!/usr/bin/perl
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
  $lvl = $lvl_map{ lc $lvl } || LOG_WARNING;
  chomp $message;
  syslog( $lvl, $message );
}

closelog();
