package boxtroll

import "time"

type BoxStatistics struct {
	TotalNum           int64     // 盲盒总数量
	TotalOriginalPrice int64     // 盲盒总原价
	TotalPrice         int64     // 盲盒爆出礼物总价值
	LastUpdateTime     time.Time // 最后一次更新时间
}

func (b *BoxStatistics) Merge(other BoxStatistics) {
	b.TotalNum += other.TotalNum
	b.TotalOriginalPrice += other.TotalOriginalPrice
	b.TotalPrice += other.TotalPrice
	b.LastUpdateTime = other.LastUpdateTime
}

func (b *BoxStatistics) Reset() {
	*b = BoxStatistics{}
}
