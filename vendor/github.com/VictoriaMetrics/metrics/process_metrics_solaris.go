//go:build solaris

// Author: Jens Elkner (C) 2025

package metrics

import (
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

/** Solaris 11.3 types deduced from /usr/include/sys/procfs.h **/
// requires go v1.18+
type uchar_t uint8 // unsigned char
type char int8     // signed char
type short int16
type ushort_t uint16
type id_t int32
type pid_t int32
type uid_t uint32
type gid_t uid_t
type taskid_t id_t
type projid_t id_t
type zoneid_t id_t
type poolid_t id_t
type uintptr_t uint64
type long int64
type ulong_t uint64
type dev_t ulong_t
type size_t ulong_t
type time_t long
type sigset_t [16]char      // we do not need those struct, so just pad
type fltset_t [16]char      // we do not need those struct, so just pad
type sysset_t [64]char      // we do not need those struct, so just pad
type lwpstatus_t [1296]char // we do not need those struct, so just pad
type lwpsinfo_t [152]char   // we do not need those struct, so just pad

type timestruc_t struct {
	tv_sec  time_t
	tv_nsec long
}

/* process status file:  /proc/<pid>/status */
type pstatus_t struct {
	pr_flags   int32 /* flags (see below) */
	pr_nlwp    int32 /* number of active lwps in the process */
	pr_pid     pid_t /* process id */
	pr_ppid    pid_t /* parent process id */
	pr_pgid    pid_t /* process group id */
	pr_sid     pid_t /* session id */
	pr_aslwpid id_t  /* historical; now always zero */
	pr_agentid id_t  /* lwp id of the /proc agent lwp, if any */
	// 32
	pr_sigpend sigset_t  /* set of process pending signals */
	pr_brkbase uintptr_t /* address of the process heap */
	pr_brksize size_t    /* size of the process heap, in bytes */
	// 64
	pr_stkbase uintptr_t   /* address of the process stack */
	pr_stksize size_t      /* size of the process stack, in bytes */
	pr_utime   timestruc_t /* # process user cpu time */
	// 96
	pr_stime  timestruc_t /* # process system cpu time */
	pr_cutime timestruc_t /* # sum of children's user times */
	// 128
	pr_cstime   timestruc_t /* # sum of children's system times */
	pr_sigtrace sigset_t    /* sigset_t: set of traced signals */
	// 160
	pr_flttrace fltset_t /* set of traced faults */
	pr_sysentry sysset_t /* set of system calls traced on entry */
	// 240
	pr_sysexit sysset_t /* set of system calls traced on exit */
	// 304
	pr_dmodel    char    /* data model of the process (see below) */
	pr_va_mask   uchar_t /* VA masking bits, where supported */
	pr_adi_nbits uchar_t /* # of VA bits used by ADI when enabled */
	pr_pad       [1]char
	pr_taskid    taskid_t /* task id */
	// 312
	pr_projid projid_t /* project id */
	pr_nzomb  int32    /* number of zombie lwps in the process */
	pr_zoneid zoneid_t /* zone id */
	// 324
	pr_filler [15]int32 /* reserved for future use */
	// 384
	pr_lwp lwpstatus_t /* status of the representative lwp */
	// 1680
}

const PRARGSZ = 80 /* number of chars of arguments */
const PRFNSZ = 16  /* Maximum size of execed filename */

/* process ps(1) information file.  /proc/<pid>/psinfo */
type psinfo_t struct {
	pr_flag int32 /* process flags (DEPRECATED; do not use) */
	pr_nlwp int32 /* number of active lwps in the process */
	pr_pid  pid_t /* unique process id */
	pr_ppid pid_t /* process id of parent */
	pr_pgid pid_t /* pid of process group leader */
	pr_sid  pid_t /* session id */
	pr_uid  uid_t /* real user id */
	pr_euid uid_t /* effective user id */
	// 32
	pr_gid    gid_t     /* real group id */
	pr_egid   gid_t     /* effective group id */
	pr_addr   uintptr_t /* address of process */
	pr_size   size_t    /* size of process image in Kbytes */
	pr_rssize size_t    /* resident set size in Kbytes */
	// 64
	pr_rssizepriv size_t /* resident set size of private mappings */
	pr_ttydev     dev_t  /* controlling tty device (or PRNODEV) */
	/* The following percent numbers are 16-bit binary */
	/* fractions [0 .. 1] with the binary point to the */
	/* right of the high-order bit (1.0 == 0x8000) */
	pr_pctcpu ushort_t /* % of recent cpu time used by all lwps */
	pr_pctmem ushort_t /* % of system memory used by process */
	pr_dummy  int32    /* 8 byte alignment: GO doesn't do it automagically */
	// 84 + 4 = 88
	pr_start timestruc_t /* process start time, from the epoch */
	pr_time  timestruc_t /* usr+sys cpu time for this process */
	pr_ctime timestruc_t /* usr+sys cpu time for reaped children */
	// 136
	pr_fname  [PRFNSZ]char  /* name of execed file */
	pr_psargs [PRARGSZ]char /* initial characters of arg list */
	// 232
	pr_wstat  int32     /* if zombie, the wait() status */
	pr_argc   int32     /* initial argument count */
	pr_argv   uintptr_t /* address of initial argument vector */
	pr_envp   uintptr_t /* address of initial environment vector */
	pr_dmodel char      /* data model of the process */
	pr_pad2   [3]char
	pr_taskid taskid_t /* task id */
	// 264
	pr_projid   projid_t /* project id */
	pr_nzomb    int32    /* number of zombie lwps in the process */
	pr_poolid   poolid_t /* pool id */
	pr_zoneid   zoneid_t /* zone id */
	pr_contract id_t     /* process contract */
	pr_filler   [1]int32 /* reserved for future use */
	// 288
	pr_lwp lwpsinfo_t /* information for representative lwp */
	// 440
}

/* Resource usage.  /proc/<pid>/usage /proc/<pid>/lwp/<lwpid>/lwpusage */
type prusage_t struct {
	pr_lwpid id_t  /* lwp id.  0: process or defunct */
	pr_count int32 /* number of contributing lwps */
	// 8
	pr_tstamp timestruc_t /* current time stamp */
	pr_create timestruc_t /* process/lwp creation time stamp */
	pr_term   timestruc_t /* process/lwp termination time stamp */
	pr_rtime  timestruc_t /* total lwp real (elapsed) time */
	// 72
	pr_utime  timestruc_t /* user level cpu time */
	pr_stime  timestruc_t /* system call cpu time */
	pr_ttime  timestruc_t /* other system trap cpu time */
	pr_tftime timestruc_t /* text page fault sleep time */
	// 136
	pr_dftime  timestruc_t /* data page fault sleep time */
	pr_kftime  timestruc_t /* kernel page fault sleep time */
	pr_ltime   timestruc_t /* user lock wait sleep time */
	pr_slptime timestruc_t /* all other sleep time */
	// 200
	pr_wtime    timestruc_t /* wait-cpu (latency) time */
	pr_stoptime timestruc_t /* stopped time */
	// 232
	filltime [6]timestruc_t /* filler for future expansion */
	// 328
	pr_minf  ulong_t /* minor page faults */
	pr_majf  ulong_t /* major page faults */
	pr_nswap ulong_t /* swaps */
	pr_inblk ulong_t /* input blocks (JEL: disk events not always recorded, so perhaps usable as an indicator but not more) */
	// 360
	pr_oublk ulong_t /* output blocks (JEL: disk events not always recorded, so perhaps usable as an indicator but not more) */
	pr_msnd  ulong_t /* messages sent */
	pr_mrcv  ulong_t /* messages received */
	pr_sigs  ulong_t /* signals received */
	// 392
	pr_vctx ulong_t /* voluntary context switches */
	pr_ictx ulong_t /* involuntary context switches */
	pr_sysc ulong_t /* system calls */
	pr_ioch ulong_t /* chars read and written (JEL: no matter, whether to/from disk or somewhere else) */
	// 424
	filler [10]ulong_t /* filler for future expansion */
	// 504
}

/** End Of Solaris types **/

type ProcMetric uint32

const (
	PM_OPEN_FDS ProcMetric = iota
	PM_MAX_FDS
	PM_MINFLT
	PM_MAJFLT
	PM_CPU_UTIL
	PM_MEM_UTIL
	PM_CMINFLT // Linux, only
	PM_CMAJFLT // Linux, only
	PM_UTIME
	PM_STIME
	PM_TIME
	PM_CUTIME
	PM_CSTIME
	PM_CTIME
	PM_NUM_THREADS
	PM_STARTTIME
	PM_VSIZE
	PM_RSS
	PM_VCTX
	PM_ICTX
	PM_BLKIO // Linux, only
	PM_COUNT /* contract: must be the last one */
)

type MetricInfo struct {
	name, help, mtype string
}

/* process metric names and descriptions */
var pm_desc = [PM_COUNT]MetricInfo{
	{ // PM_OPEN_FDS
		"process_open_fds",
		"Number of open file descriptors",
		"gauge",
	}, { // PM_MAX_FDS
		"process_max_fds",
		"Max. number of open file descriptors (soft limit)",
		"gauge",
	}, { // PM_MINFLT
		"process_minor_pagefaults",
		"Number of minor faults of the process not caused a page load from disk",
		"counter",
	}, { // PM_MAJFLT
		"process_major_pagefaults",
		"Number of major faults of the process caused a page load from disk",
		"counter",
	}, { // PM_CPU_UTIL
		"process_cpu_utilization_percent",
		"Percent of recent cpu time used by all lwps",
		"gauge",
	}, { // PM_MEM_UTIL
		"process_mem_utilization_percent",
		"Percent of system memory used by process",
		"gauge",
	}, { // PM_CMINFLT
		"process_children_minor_pagefaults",
		"Number of minor faults of the process waited-for children not caused a page load from disk",
		"counter",
	}, { // PM_CMAJFLT
		"process_children_major_pagefaults",
		"Number of major faults of the process's waited-for children caused a page load from disk",
		"counter",
	}, { // PM_UTIME
		"process_user_cpu_seconds",
		"Total CPU time the process spent in user mode in seconds",
		"counter",
	}, { // PM_STIME
		"process_system_cpu_seconds",
		"Total CPU time the process spent in kernel mode in seconds",
		"counter",
	}, { // PM_TIME
		"process_total_cpu_seconds",
		"Total CPU time the process spent in user and kernel mode in seconds",
		"counter",
	}, { // PM_CUTIME
		"process_children_user_cpu_seconds",
		"Total CPU time the process's waited-for children spent in user mode in seconds",
		"counter",
	}, { // PM_CSTIME
		"process_children_system_cpu_seconds",
		"Total CPU time the process's waited-for children spent in kernel mode in seconds",
		"counter",
	}, { // PM_CTIME
		"process_children_total_cpu_seconds",
		"Total CPU time the process's waited-for children spent in user and in kernel mode in seconds",
		"counter",
	}, { // PM_NUM_THREADS
		"process_threads_total",
		"Number of threads in this process",
		"gauge",
	}, { // PM_STARTTIME
		"process_start_time_seconds",
		"The time the process has been started in seconds elapsed since Epoch",
		"counter",
	}, { // PM_VSIZE
		"process_virtual_memory_bytes",
		"Virtual memory size in bytes",
		"gauge",
	}, { // PM_RSS
		"process_resident_memory_bytes",
		"Resident set size of memory in bytes",
		"gauge",
	}, { // PM_VCTX
		"process_voluntary_ctxsw_total",
		"Number of voluntary context switches",
		"counter",
	}, { // PM_ICTX
		"process_involuntary_ctxsw_total",
		"Number of involuntary context switches",
		"counter",
	}, { // PM_BLKIO
		"process_delayacct_blkio_ticks",
		"Aggregated block I/O delays, measured in clock ticks (centiseconds)",
		"counter",
	},
}

type ProcFd uint32

const (
	FD_LIMITS ProcFd = iota
	FD_STAT
	FD_PSINFO // solaris/illumos, only
	FD_USAGE  // solaris/illumos, only
	FD_COUNT  /* contract: must be the last one */
)

/* emittable process metrics for solaris */
var activeProcMetrics = []ProcMetric{
	PM_MINFLT,
	PM_MAJFLT,
	PM_CPU_UTIL,
	PM_MEM_UTIL,
	PM_UTIME,
	PM_STIME,
	PM_TIME,
	PM_CUTIME,
	PM_CSTIME,
	PM_CTIME,
	PM_NUM_THREADS,
	PM_STARTTIME,
	PM_VSIZE,
	PM_RSS,
	PM_VCTX,
	PM_ICTX,
}

/* emittable fd metrics for solaris */
var activeFdMetrics = []ProcMetric{
	PM_OPEN_FDS,
	PM_MAX_FDS,
}

/*
process metrics related file descriptors for files we always need, and

	do not want to open/close all the time
*/
var pm_fd [FD_COUNT]int

/*
to avaid, that go closes the files in the background, which makes the FDs

	above useless, we need to keep the reference to them as well
*/
var pm_file [FD_COUNT]*os.File

/*
process metric values. TSDBs use internally always float64, so we do not

	need to make a difference between int and non-int values
*/
var pm_val [PM_COUNT]float64

/* path used to count open FDs */
var fd_path string

/* lazy init of this process related metrics */
func init() {
	var testdata_dir = ""
	var onTest = len(os.Args) > 1 && strings.HasSuffix(os.Args[0], ".test")
	if onTest {
		cwd, err := os.Getwd()
		if err != nil {
			panic("Unknwon directory: " + err.Error())
		}
		testdata_dir = cwd + "/testdata"
		fmt.Printf("Using test data in %s ...\n", testdata_dir)
	}

	// we preset all so that it is safe to use these vals even if the rest of
	// init fails
	for i := 0; i < int(PM_COUNT); i++ {
		pm_val[i] = 0
	}
	for i := 0; i < int(FD_COUNT); i++ {
		pm_fd[i] = -1
	}
	pid := os.Getpid()
	if onTest {
		fd_path = testdata_dir + "/fd"
	} else {
		fd_path = fmt.Sprintf("/proc/%d/fd", pid)
	}

	// NOTE: We do NOT close these FDs intentionally to avoid the open/close
	// overhead for each update.
	var path string
	if onTest {
		path = fmt.Sprintf(testdata_dir + "/solaris.ps_status")
	} else {
		path = fmt.Sprintf("/proc/%d/status", pid)
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		log.Printf("ERROR: metrics: Unable to open %s (%v).", path, err)
	} else {
		pm_file[FD_STAT] = f
		pm_fd[FD_STAT] = int(f.Fd())
	}
	if onTest {
		path = fmt.Sprintf(testdata_dir + "/solaris.ps_info")
	} else {
		path = fmt.Sprintf("/proc/%d/psinfo", pid)
	}
	f, err = os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		log.Printf("ERROR: metrics: Unable to open %s (%v).", path, err)
	} else {
		pm_file[FD_PSINFO] = f
		pm_fd[FD_PSINFO] = int(f.Fd())
	}
	if onTest {
		path = fmt.Sprintf(testdata_dir + "/solaris.ps_usage")
	} else {
		path = fmt.Sprintf("/proc/%d/usage", pid)
	}
	f, err = os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		log.Printf("ERROR: metrics: Unable to open %s (%v).", path, err)
	} else {
		pm_file[FD_USAGE] = f
		pm_fd[FD_USAGE] = int(f.Fd())
	}

	/* usually an app does|cannot not change its own FD limits. So we handle
	it as a const - determine it once, only */
	var lim syscall.Rlimit
	err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim)
	if err == nil {
		pm_val[PM_MAX_FDS] = float64(lim.Cur)
	} else {
		log.Printf("ERROR: metrics: Unable determin max. fd limit.")
		pm_val[PM_MAX_FDS] = -1
	}
}

var nan = math.NaN()

func time2float(t timestruc_t) float64 {
	return float64(t.tv_sec) + float64(t.tv_nsec)*1e-9
}
func time2float2(a timestruc_t, b timestruc_t) float64 {
	return float64(a.tv_sec+b.tv_sec) + float64(a.tv_nsec+b.tv_nsec)*1e-9
}

func updateProcMetrics() {
	var status pstatus_t
	var psinfo psinfo_t
	var usage prusage_t

	var fail = pm_fd[FD_STAT] < 0
	if !fail {
		n, err := syscall.Pread(pm_fd[FD_STAT],
			(*(*[unsafe.Sizeof(status)]byte)(unsafe.Pointer(&status)))[:], 0)
		fail = (n < 324 || err != nil)
		if fail {
			fmt.Printf("WARNING: read %s@%d failed: %v\n",
				pm_file[FD_STAT].Name(), n, err)
		}
	}
	if fail {
		pm_val[PM_NUM_THREADS] = nan
		pm_val[PM_UTIME] = nan
		pm_val[PM_STIME] = nan
		pm_val[PM_TIME] = nan
		pm_val[PM_CUTIME] = nan
		pm_val[PM_CSTIME] = nan
		pm_val[PM_CTIME] = nan
	} else {
		pm_val[PM_NUM_THREADS] = float64(status.pr_nlwp + status.pr_nzomb)
		pm_val[PM_UTIME] = time2float(status.pr_utime)
		pm_val[PM_STIME] = time2float(status.pr_stime)
		pm_val[PM_TIME] = time2float2(status.pr_utime, status.pr_stime)
		pm_val[PM_CUTIME] = time2float(status.pr_cutime)
		pm_val[PM_CSTIME] = time2float(status.pr_cstime)
		pm_val[PM_CTIME] = time2float2(status.pr_cutime, status.pr_cstime)
	}
	fail = pm_fd[FD_PSINFO] < 0
	if !fail {
		n, err := syscall.Pread(pm_fd[FD_PSINFO],
			(*(*[unsafe.Sizeof(psinfo)]byte)(unsafe.Pointer(&psinfo)))[:], 0)
		fail = (n < 272 || err != nil)
		if fail {
			fmt.Printf("WARNING: read %s@%d failed: %v\n",
				pm_file[FD_PSINFO].Name(), n, err)
		}
	}
	if fail {
		pm_val[PM_VSIZE] = nan
		pm_val[PM_RSS] = nan
		pm_val[PM_CPU_UTIL] = nan
		pm_val[PM_MEM_UTIL] = nan
		pm_val[PM_STARTTIME] = nan
	} else {
		//num_threads = psinfo.pr_nlwp + psinfo.pr_nzomb	// already by status
		pm_val[PM_VSIZE] = float64(psinfo.pr_size << 10)
		pm_val[PM_RSS] = float64(psinfo.pr_rssize << 10)
		pm_val[PM_CPU_UTIL] = 100 * float64(psinfo.pr_pctcpu) / float64(0x8000)
		pm_val[PM_MEM_UTIL] = 100 * float64(psinfo.pr_pctmem) / float64(0x8000)
		pm_val[PM_STARTTIME] = float64(psinfo.pr_start.tv_sec)
	}
	fail = pm_fd[FD_USAGE] < 0
	if !fail {
		n, err := syscall.Pread(pm_fd[FD_USAGE],
			(*(*[unsafe.Sizeof(usage)]byte)(unsafe.Pointer(&usage)))[:], 0)
		fail = (n < 424 || err != nil)
		if fail {
			fmt.Printf("WARNING: read %s@%d failed: %v\n",
				pm_file[FD_USAGE].Name(), n, err)
		}
	}
	if fail {
		pm_val[PM_MINFLT] = nan
		pm_val[PM_MAJFLT] = nan
		pm_val[PM_VCTX] = nan
		pm_val[PM_ICTX] = nan
	} else {
		pm_val[PM_MINFLT] = float64(usage.pr_minf)
		pm_val[PM_MAJFLT] = float64(usage.pr_majf)
		pm_val[PM_VCTX] = float64(usage.pr_vctx)
		pm_val[PM_ICTX] = float64(usage.pr_ictx)
	}
}

func updateFdMetrics() {
	pm_val[PM_OPEN_FDS] = 0
	f, err := os.Open(fd_path)
	if err != nil {
		log.Printf("ERROR: metrics: Unable to open %s", fd_path)
		return
	}
	defer f.Close()
	for {
		names, err := f.Readdirnames(512)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("ERROR: metrics: Read error for %s: %s", fd_path, err)
			return
		}
		pm_val[PM_OPEN_FDS] += float64(len(names))
	}
}

func writeProcessMetrics(w io.Writer) {
	updateProcMetrics()
	if isMetadataEnabled() {
		for _, v := range activeProcMetrics {
			fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n%s %.17g\n",
				pm_desc[v].name, pm_desc[v].help,
				pm_desc[v].name, pm_desc[v].mtype,
				pm_desc[v].name, pm_val[v])
		}
	} else {
		for _, v := range activeProcMetrics {
			fmt.Fprintf(w, "%s %.17g\n", pm_desc[v].name, pm_val[v])
		}
	}
}

func writeFDMetrics(w io.Writer) {
	updateFdMetrics()
	if isMetadataEnabled() {
		for _, v := range activeFdMetrics {
			fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n%s %.17g\n",
				pm_desc[v].name, pm_desc[v].help,
				pm_desc[v].name, pm_desc[v].mtype,
				pm_desc[v].name, pm_val[v])
		}
	} else {
		for _, v := range activeFdMetrics {
			fmt.Fprintf(w, "%s %.17g\n", pm_desc[v].name, pm_val[v])
		}
	}
}
