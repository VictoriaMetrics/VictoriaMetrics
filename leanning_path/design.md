# VictoriaMetrics 设计
## 1. 背景

VictoriaMetrics(简称vm)的内部设计文档过于稀少，为了方便日后的维护和问题排查，尝试从源码分析其每个组件的核心功能

## 2. 组件

### 2.1 vmagent

// TODO

### 2.2 vmselect

// TODO

### 2.3 vmstorage

![rowRowsShards design](./ssr.png)

有兴趣了解这个实现可以参考这个MR [merge request](https://kgit.kugou.net/yongquanli/VictoriaMetrics/-/tree/rrs_implement)

生成partition后的内存压缩逻辑 [compression](https://segmentfault.com/a/1190000043749609)

代码路径：

```shell
raw_row.go#marshalToInmemoryPart
```

uint64的压缩手段，跟leveldb的实现方式一样，每个byte的最高位用来表示这个整数的开头的第一个byte。
所以一个byte的有效存储为7个bit，对于uint64这样8个byte的整数，最多需要5个byte来存储。
```go
// MarshalVarUint64 appends marshaled u to dst and returns the result.
func MarshalVarUint64(dst []byte, u uint64) []byte {
if u < (1 << 7) {
return append(dst, byte(u))
}
if u < (1 << (2 * 7)) {
return append(dst, byte(u|0x80), byte(u>>7))
}
if u < (1 << (3 * 7)) {
return append(dst, byte(u|0x80), byte((u>>7)|0x80), byte(u>>(2*7)))
}

// Slow path for big integers.
var tmp [1]uint64
tmp[0] = u
return MarshalVarUint64s(dst, tmp[:])
}
```

- 压缩

压缩的逻辑稍显复杂，但是核心有几个点

- 相同的指标（TSID）放在同一个block里面(block header)会维护这个信息
- block内根据timestamp进行排序
- values值和timestamp分别存储在两个array里面，这样方便对相邻的值进行压缩
- 有一个思路是相邻的值的差异不会太大（指标数据的特点），一般是上下浮动。因此可以考虑计算相邻两值的diff值进行存储
- 我们只存储diff值，diff值考虑用变长整形来存储（eg. varUnit64），可以大大减少存储空间
- block header本质上也是个索引，保存了每个block的最早的timestamp和最晚的timestamp方便进行检索


### 2.4 vminsert



