package nfsstats

// NFS mountstat documentation from http://www.fsl.cs.stonybrook.edu/~mchen/mountstat-format.txt
// We will only support statvers=1.1
import (
    "bufio"
    "io"
    "strconv"
    "strings"
)

// Full NFS mount object
type NFSMount struct {
    Device string
    Mountpoint string
    Statistics *Statistics
    Version uint64
}

// NFS statistics wrapper object
type Statistics struct {
    Age uint64
    Byte ByteCounters
    Event EventCounters
    Operation map[string]*OperationCounters
    Transport TransportCounters
}

// Byte (linux/nfs_iostat.h: nfs_stat_bytecounters)
// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/include/linux/nfs_iostat.h#n27
/*
 * NFS byte counters
 *
 * 1.  SERVER - the number of payload bytes read from or written
 *              to the server by the NFS client via an NFS READ or WRITE
 *              request.
 *
 * 2.  NORMAL - the number of bytes read or written by applications
 *              via the read(2) and write(2) system call interfaces.
 *
 * 3.  DIRECT - the number of bytes read or written from files
 *              opened with the O_DIRECT flag.
 *
 * These counters give a view of the data throughput into and out
 * of the NFS client.  Comparing the number of bytes requested by
 * an application with the number of bytes the client requests from
 * the server can provide an indication of client efficiency
 * (per-op, cache hits, etc).
 *
 * These counters can also help characterize which access methods
 * are in use.  DIRECT by itself shows whether there is any O_DIRECT
 * traffic.  NORMAL + DIRECT shows how much data is going through
 * the system call interface.  A large amount of SERVER traffic
 * without much NORMAL or DIRECT traffic shows that applications
 * are using mapped files.
 *
 * NFS page counters
 *
 * These count the number of pages read or written via nfs_readpage(),
 * nfs_readpages(), or their write equivalents.
 *
 * NB: When adding new byte counters, please include the measured
 * units in the name of each byte counter to help users of this
 * interface determine what exactly is being counted.
 */
type ByteCounters struct {
    NormalReadBytes, NormalWriteBytes, DirectReadBytes, DirectWriteBytes,
    ServerReadBytes, ServerWriteBytes, ReadPages, WritePages uint64
}

// Event (linux/nfs_iostat.h: nfs_stat_eventcounters)
// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/include/linux/nfs_iostat.h#n74
/*
 * NFS event counters
 *
 * These counters provide a low-overhead way of monitoring client
 * activity without enabling NFS trace debugging.  The counters
 * show the rate at which VFS requests are made, and how often the
 * client invalidates its data and attribute caches.  This allows
 * system administrators to monitor such things as how close-to-open
 * is working, and answer questions such as "why are there so many
 * GETATTR requests on the wire?"
 *
 * They also count anamolous events such as short reads and writes,
 * silly renames due to close-after-delete, and operations that
 * change the size of a file (such operations can often be the
 * source of data corruption if applications aren't using file
 * locking properly).
 */
type EventCounters struct {
    InodeRevalidate, DentryRevalidate, DataInvalidate, AttrInvalidate,
    VFSOpen, VFSLookup, VFSAccess, VFSUpdatePage, VFSReadPage, VFSReadPages,
    VFSWritePage, VFSWritePages, VFSGetDents, VFSSetAttr, VFSFlush, VFSSync,
    VFSLock, VFSRelease, CongestionWait, SetAttrTrunc, ExtendWrite,
    SillyRename, ShortRead, ShortWrite, Delay, PNFSRead, PNFSWrite uint64
}

// Operation (linux/net/sunrpc/stats.c: rpc_count_iostats_metrics)
// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/net/sunrpc/stats.c#n149
type OperationCounters struct {
    Requests, Transmissions, Timeouts, BytesSent, BytesReceived, TotalQueueTime,
    TotalResponseTime, TotalExecutionTime uint64
}

// Transport (linux/net/sunrpc/xprtsock.c: xs_tcp_print_stats)
// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/net/sunrpc/xprtsock.c#n2600
type TransportCounters struct {
    SourcePort, BindCount, ConnectCount, ConnectTime, IdleTime, RPCSends,
    RPCReceives, BadTransactionIDs, RequestUtilization, BacklogUtilization, MaxSlotsUsed,
    SendingQueueUtilization, PendingQueueUtilization uint64
}

// Parse the contents of a /proc/:pid/mountstats file and return stats about each NFS mount
func Parse(reader io.Reader) ([]*NFSMount, error) {
    var nfsMounts []*NFSMount

    scanner := bufio.NewScanner(reader)

    // Loop through each line and extract mount data
    for scanner.Scan() {
        // Split each line on spaces
        fields := strings.Fields(string(scanner.Bytes()))

        // Skip empty lines
        if len(fields) == 0 { continue }

        // EXAMPLE 1: "device /dev/sdb1 mounted on /boot with fstype ext2"
        // EXAMPLE 2: "device 192.168.253.5:/srv/fatman/ctdata mounted on /data/ctwatch/db with fstype nfs statvers=1.1"
        // Most mounts have 8 fields, mounts with stats (like NFS) have 9 fields and the last is "statsver={version}"

        if fields[0] == "device" {
            // Skip mounts that are not NFS and not version 1.1
            if len(fields) != 9 { continue }
            if fields[7] != "nfs" && fields[7] != "nfs4" { continue }
            if fields[8] != "statvers=1.1" { continue }

            // Determine NFS version (8th element)
            nfsVersion := 3
            if fields[7] == "nfs4" {
                nfsVersion = 4
            }

            // Save the basic mount info
            nfsMount := &NFSMount {
                Device: fields[1],
                Mountpoint: fields[4],
                Version: uint64(nfsVersion),
            }

            // Capture statistics
            statistics, err := parseStatistics(scanner)
            if err != nil {
                return nil, err
            }
            nfsMount.Statistics = statistics

            // Add it to the pile
            nfsMounts = append(nfsMounts, nfsMount)
        }
    }

    return nfsMounts, scanner.Err()
}

func NewStatistics() *Statistics {
    var statistics Statistics
    statistics.Operation = make(map[string]*OperationCounters)
    return &statistics
}

func parseStatistics(scanner *bufio.Scanner) (*Statistics, error) {
    statistics := NewStatistics()

    // Extract each metric type
    for scanner.Scan() {
        // Split each line on spaces
        fields := strings.Fields(string(scanner.Bytes()))

        // The stats are done or we need to move on to per-operation stats
        // Either way we leave the loop
        if len(fields) == 0 { break }

        // Bail if per-operation stats
        if fields[0] == "per-op" { break }

        // Determine stats type and parse it
        switch fields[0] {
            case "age:":
                statistics.Age, _ = strconv.ParseUint(fields[1], 10, 64)

            case "bytes:":
                // There must be 9 byte elements
                if len(fields) != 9 { continue }

                elements := makeUint64(fields[1:])

                statistics.Byte = ByteCounters {
                    NormalReadBytes: elements[0],
                    NormalWriteBytes: elements[1],
                    DirectReadBytes: elements[2],
                    DirectWriteBytes: elements[3],
                    ServerReadBytes: elements[4],
                    ServerWriteBytes: elements[5],
                    ReadPages: elements[6],
                    WritePages: elements[7],
                }

            case "events:":
                // There must be 28 event elements
                if len(fields) != 28 { continue }

                elements := makeUint64(fields[1:])

                statistics.Event = EventCounters {
                    InodeRevalidate: elements[0],
                    DentryRevalidate: elements[1],
                    DataInvalidate: elements[2],
                    AttrInvalidate: elements[3],
                    VFSOpen: elements[4],
                    VFSLookup: elements[5],
                    VFSAccess: elements[6],
                    VFSUpdatePage: elements[7],
                    VFSReadPage: elements[8],
                    VFSReadPages: elements[9],
                    VFSWritePage: elements[10],
                    VFSWritePages: elements[11],
                    VFSGetDents: elements[12],
                    VFSSetAttr: elements[13],
                    VFSFlush: elements[14],
                    VFSSync: elements[15],
                    VFSLock: elements[16],
                    VFSRelease: elements[17],
                    CongestionWait: elements[18],
                    SetAttrTrunc: elements[19],
                    ExtendWrite: elements[20],
                    SillyRename: elements[21],
                    ShortRead: elements[22],
                    ShortWrite: elements[23],
                    Delay: elements[24],
                    PNFSRead: elements[25],
                    PNFSWrite: elements[26],
                }

            case "xprt:":
                // We only parse it if the transport is TCP
                // Based on docs, it looks like UDP doesn't report this line
                // FIXME: check against udp
                if fields[1] != "tcp" { continue }

                // There must be 15 transport elements
                if len(fields) != 15 { continue }

                elements := makeUint64(fields[2:])

                statistics.Transport = TransportCounters {
                    SourcePort: elements[0],
                    BindCount: elements[1],
                    ConnectCount: elements[2],
                    ConnectTime: elements[3],
                    IdleTime: elements[4],
                    RPCSends: elements[5],
                    RPCReceives: elements[6],
                    BadTransactionIDs: elements[7],
                    RequestUtilization: elements[8],
                    BacklogUtilization: elements[9],
                    MaxSlotsUsed: elements[10],
                    SendingQueueUtilization: elements[11],
                    PendingQueueUtilization: elements[12],
                }
        }
    }

    // Extract per-operation stats
    parseOperations(scanner, statistics)
    if err := scanner.Err(); err != nil {
        return nil, err
    }

    // We're done here
    return statistics, nil
}

func parseOperations(scanner *bufio.Scanner, statistics *Statistics) () {
    // Extract each metric type
    for scanner.Scan() {
        // Split each line on spaces
        fields := strings.Fields(string(scanner.Bytes()))
        // Bail if the line is empty or a device line is encountered
        if len(fields) == 0 || fields[0] == "device" { break }
        // Skip malformed lines
        if len(fields) != 9 { continue }

        // Store the values
        opName := strings.TrimSuffix(fields[0], ":")

        elements := makeUint64(fields[1:])
        statistics.Operation[opName] = &OperationCounters {
            Requests: elements[0],
            Transmissions: elements[1],
            Timeouts: elements[2],
            BytesSent: elements[3],
            BytesReceived: elements[4],
            TotalQueueTime: elements[5],
            TotalResponseTime: elements[6],
            TotalExecutionTime: elements[7],
        }
    }

    return
}

func makeUint64(fields []string) []uint64 {
    // Iterate over each field element and re-cast it from string to uint64
    elements := make([]uint64, 0, len(fields))
    for _, element := range fields {
        val, _ := strconv.ParseUint(element, 10, 64)
        elements = append(elements, val)
    }

    return elements
}

// vim:ft=go:et:ts=4:sw=4:sts=4:

