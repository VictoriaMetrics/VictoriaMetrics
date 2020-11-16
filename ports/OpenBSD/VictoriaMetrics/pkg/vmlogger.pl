#!/usr/bin/perl
use Sys::Syslog qw(:standard :macros);

openlog($ARGV[0], "pid", "daemon");

while (my $l = <STDIN>) {
  my @d = split /\t/, $l;
  # go level : "INFO", "WARN", "ERROR", "FATAL", "PANIC":
  my $lvl = $d[0];
  $lvl = LOG_EMERG if ($lvl eq 'panic');
  $lvl = 'crit' if ($lvl eq 'fatal');
  $lvl = 'err' if ($lvl eq 'error');
  $lvl = 'warning' if ($lvl eq 'warn');
  chomp $d[2];
  syslog( $lvl, $d[2] );
}

closelog();
