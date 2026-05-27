package usertraffic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	fiveMinSec    = 300
	hourSec       = 3600
	retainYear    = 365 * 24 * time.Hour
	retainFiveMin = 7 * 24 * time.Hour
)

// Bucket 一段时间内的累计字节（非瞬时速率）
type Bucket struct {
	Ts      int64  `json:"ts"`
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}

// PeerSeries 单 Peer 流量历史
type PeerSeries struct {
	FiveMin []Bucket `json:"five_min,omitempty"`
	Hourly  []Bucket `json:"hourly,omitempty"`
	LastRx  uint64   `json:"last_rx,omitempty"`
	LastTx  uint64   `json:"last_tx,omitempty"`
}

// Summary 统计摘要
type Summary struct {
	TotalRxBytes  uint64 `json:"total_rx_bytes"`
	TotalTxBytes  uint64 `json:"total_tx_bytes"`
	TodayRxBytes  uint64 `json:"today_rx_bytes"`
	TodayTxBytes  uint64 `json:"today_tx_bytes"`
	PeriodRxBytes uint64 `json:"period_rx_bytes"`
	PeriodTxBytes uint64 `json:"period_tx_bytes"`
}

// Point 图表点（平均 Mbps）
type Point struct {
	Ts     int64   `json:"ts"`
	RxMbps float64 `json:"rx_mbps"`
	TxMbps float64 `json:"tx_mbps"`
}

// Response API 返回（与 ocserv 用户流量结构对齐，主键为 peer 名称）
type Response struct {
	Peer       string         `json:"peer"`
	Online     bool           `json:"online"`
	Current    map[string]any `json:"current,omitempty"`
	Summary    Summary        `json:"summary"`
	Series     []Point        `json:"series"`
	Resolution string         `json:"resolution"` // 5min | hourly
	Period     string         `json:"period"`
}

type dbFile struct {
	Peers map[string]*PeerSeries `json:"peers"`
}

// Store 持久化 WireGuard Peer 流量（保留约 1 年 hourly + 近 7 日 5min）
type Store struct {
	path string
	mu   sync.Mutex
	data dbFile
}

var defaultStore *Store

// DefaultStore 单例
func DefaultStore() *Store {
	if defaultStore == nil {
		defaultStore = NewStore(filepath.Join("/var/lib/qosnat2", "wireguard-peer-traffic.json"))
	}
	return defaultStore
}

// NewStore 指定存储路径（测试用）
func NewStore(path string) *Store {
	s := &Store{path: path, data: dbFile{Peers: map[string]*PeerSeries{}}}
	_ = s.load()
	return s
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var f dbFile
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	if f.Peers == nil {
		f.Peers = map[string]*PeerSeries{}
	}
	s.data = f
	return nil
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func bucketTs(t time.Time, intervalSec int64) int64 {
	unix := t.Unix()
	return unix - (unix % intervalSec)
}

func (s *Store) peerSeries(name string) *PeerSeries {
	if s.data.Peers == nil {
		s.data.Peers = map[string]*PeerSeries{}
	}
	u, ok := s.data.Peers[name]
	if !ok {
		u = &PeerSeries{}
		s.data.Peers[name] = u
	}
	return u
}

func addToBucket(buckets *[]Bucket, ts int64, rx, tx uint64) {
	if rx == 0 && tx == 0 {
		return
	}
	n := len(*buckets)
	if n > 0 && (*buckets)[n-1].Ts == ts {
		(*buckets)[n-1].RxBytes += rx
		(*buckets)[n-1].TxBytes += tx
		return
	}
	*buckets = append(*buckets, Bucket{Ts: ts, RxBytes: rx, TxBytes: tx})
}

func pruneBuckets(buckets *[]Bucket, minTs int64) {
	i := 0
	for _, b := range *buckets {
		if b.Ts >= minTs {
			(*buckets)[i] = b
			i++
		}
	}
	*buckets = (*buckets)[:i]
}

// RecordCounters 根据 wg transfer 当前 RX/TX 计数累加增量
func (s *Store) RecordCounters(peerName string, rx, tx uint64, now time.Time) error {
	peerName = normalizePeerName(peerName)
	if peerName == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	u := s.peerSeries(peerName)
	var drx, dtx uint64
	if rx >= u.LastRx {
		drx = rx - u.LastRx
	} else {
		drx = rx
	}
	if tx >= u.LastTx {
		dtx = tx - u.LastTx
	} else {
		dtx = tx
	}
	u.LastRx = rx
	u.LastTx = tx

	if drx > 0 || dtx > 0 {
		ts5 := bucketTs(now, fiveMinSec)
		tsH := bucketTs(now, hourSec)
		addToBucket(&u.FiveMin, ts5, drx, dtx)
		addToBucket(&u.Hourly, tsH, drx, dtx)
	}

	cut5 := now.Add(-retainFiveMin).Unix()
	cutH := now.Add(-retainYear).Unix()
	pruneBuckets(&u.FiveMin, cut5)
	pruneBuckets(&u.Hourly, cutH)

	return s.saveLocked()
}

func normalizePeerName(name string) string {
	return strings.TrimSpace(name)
}

func bytesToMbps(bytes uint64, intervalSec int64) float64 {
	if bytes == 0 || intervalSec <= 0 {
		return 0
	}
	return float64(bytes) * 8 / (float64(intervalSec) * 1_000_000)
}

func sumBuckets(buckets []Bucket, since int64) (rx, tx uint64) {
	for _, b := range buckets {
		if b.Ts >= since {
			rx += b.RxBytes
			tx += b.TxBytes
		}
	}
	return rx, tx
}

func sumAll(buckets []Bucket) (rx, tx uint64) {
	for _, b := range buckets {
		rx += b.RxBytes
		tx += b.TxBytes
	}
	return rx, tx
}

// TotalBytes 返回 Peer 历史累计 RX/TX（hourly 桶汇总，保留约 1 年）
func (s *Store) TotalBytes(peerName string) (rx, tx uint64) {
	peerName = normalizePeerName(peerName)
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.data.Peers[peerName]
	if u == nil {
		return 0, 0
	}
	return sumAll(u.Hourly)
}

// Query 查询 Peer 流量与图表序列
func (s *Store) Query(peerName, period string) Response {
	peerName = normalizePeerName(peerName)
	now := time.Now()
	since, res, interval := periodBounds(period, now)

	s.mu.Lock()
	u := s.data.Peers[peerName]
	var hourly, fiveMin []Bucket
	if u != nil {
		hourly = append([]Bucket(nil), u.Hourly...)
		fiveMin = append([]Bucket(nil), u.FiveMin...)
	}
	s.mu.Unlock()

	resp := Response{
		Peer:       peerName,
		Summary:    Summary{},
		Series:     []Point{},
		Resolution: res,
		Period:     period,
	}
	if u == nil {
		return resp
	}

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	resp.Summary.TotalRxBytes, resp.Summary.TotalTxBytes = sumAll(hourly)
	resp.Summary.TodayRxBytes, resp.Summary.TodayTxBytes = sumBuckets(hourly, todayStart)
	if rx5, tx5 := sumBuckets(fiveMin, todayStart); rx5 > 0 || tx5 > 0 {
		resp.Summary.TodayRxBytes = rx5
		resp.Summary.TodayTxBytes = tx5
	}

	var buckets []Bucket
	if res == "5min" {
		buckets = fiveMin
	} else {
		buckets = hourly
	}
	for _, b := range buckets {
		if b.Ts < since {
			continue
		}
		resp.Summary.PeriodRxBytes += b.RxBytes
		resp.Summary.PeriodTxBytes += b.TxBytes
		resp.Series = append(resp.Series, Point{
			Ts:     b.Ts,
			RxMbps: roundMbps(bytesToMbps(b.RxBytes, interval)),
			TxMbps: roundMbps(bytesToMbps(b.TxBytes, interval)),
		})
	}
	if resp.Series == nil {
		resp.Series = []Point{}
	}
	return resp
}

func roundMbps(v float64) float64 {
	if v < 0.0001 {
		return 0
	}
	if v >= 100 {
		return float64(int(v*10+0.5)) / 10
	}
	if v >= 10 {
		return float64(int(v*100+0.5)) / 100
	}
	return float64(int(v*1000+0.5)) / 1000
}

func periodBounds(period string, now time.Time) (since int64, resolution string, intervalSec int64) {
	switch period {
	case "24h":
		return now.Add(-24 * time.Hour).Unix(), "5min", fiveMinSec
	case "30d":
		return now.Add(-30 * 24 * time.Hour).Unix(), "hourly", hourSec
	case "365d", "1y":
		return now.Add(-retainYear).Unix(), "hourly", hourSec
	default: // 7d
		return now.Add(-7 * 24 * time.Hour).Unix(), "5min", fiveMinSec
	}
}
