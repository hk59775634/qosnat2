package tuning

import "strconv"

// 连接跟踪内存模型（与 Linux nf_conntrack 常见占用一致）：
// - 宿主机总内存的 optimizationMemFraction（默认 50%）划为「NAT/QoS 优化专用」
// - 其中 conntrackMemFractionOfOpt（默认 75%）用于 conntrack 条目，其余留给哈希桶、socket 缓冲等
// - 单条约 384 字节（内核文档与实测常在 300–400B；16G×50%×75%/384 ≈ 1677 万，与常见网关建议一致）
const (
	optimizationMemFraction   = 0.5
	conntrackMemFractionOfOpt = 0.75
	conntrackBytesPerEntry    = 384
	conntrackBucketDivisor    = 4
	minConntrackMax           = 65536
	minConntrackBuckets       = 16384
)

// MemoryBudget 推荐时内存划分（供 API / UI 展示）
type MemoryBudget struct {
	MemTotalMB       int64 `json:"mem_total_mb"`
	OptimizationMB   int64 `json:"optimization_mb"`
	ConntrackMemMB   int64 `json:"conntrack_mem_mb"`
	ConntrackMax     int   `json:"conntrack_max"`
	ConntrackBuckets int   `json:"conntrack_buckets"`
	BytesPerEntry    int   `json:"bytes_per_entry"`
}

// ConntrackBudget 按「总内存 50% 专用优化」计算 nf_conntrack_max / buckets
func ConntrackBudget(h HostInfo) MemoryBudget {
	memMB := h.MemMB
	if memMB < 1 {
		memMB = 512
	}
	optMB := int64(float64(memMB) * optimizationMemFraction)
	ctMB := int64(float64(optMB) * conntrackMemFractionOfOpt)
	ctBytes := ctMB * 1024 * 1024
	max := int(ctBytes / conntrackBytesPerEntry)
	if max < minConntrackMax {
		max = minConntrackMax
	}
	buckets := conntrackBucketsForMax(max)
	return MemoryBudget{
		MemTotalMB:       memMB,
		OptimizationMB:   optMB,
		ConntrackMemMB:   ctMB,
		ConntrackMax:     max,
		ConntrackBuckets: buckets,
		BytesPerEntry:    conntrackBytesPerEntry,
	}
}

func conntrackBucketsForMax(max int) int {
	b := max / conntrackBucketDivisor
	if b < minConntrackBuckets {
		b = minConntrackBuckets
	}
	return roundUpPower2(b)
}

// roundUpPower2 内核 nf_conntrack_buckets 宜为 2 的幂
func roundUpPower2(n int) int {
	if n <= 1 {
		return 1
	}
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

func applyConntrackSysctl(m map[string]string, b MemoryBudget) {
	m["net.netfilter.nf_conntrack_max"] = strconv.Itoa(b.ConntrackMax)
	m["net.netfilter.nf_conntrack_buckets"] = strconv.Itoa(b.ConntrackBuckets)
	// file-max 需大于并发连接数（含 socket / 其它 fd）
	fileMax := b.ConntrackMax * 2
	if fileMax < 524288 {
		fileMax = 524288
	}
	m["fs.file-max"] = strconv.Itoa(fileMax)
}

// neigh thresholds scale with conntrack scale (LAN 主机数量)
func scaleNeigh(m map[string]string, max int) {
	t1, t2, t3 := 1024, 2048, 4096
	switch {
	case max >= 4000000:
		t1, t2, t3 = 8192, 16384, 32768
	case max >= 1000000:
		t1, t2, t3 = 4096, 8192, 16384
	case max >= 262144:
		t1, t2, t3 = 2048, 4096, 8192
	}
	m["net.ipv4.neigh.default.gc_thresh1"] = strconv.Itoa(t1)
	m["net.ipv4.neigh.default.gc_thresh2"] = strconv.Itoa(t2)
	m["net.ipv4.neigh.default.gc_thresh3"] = strconv.Itoa(t3)
}
