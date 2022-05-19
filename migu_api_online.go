package ula

type (
	// 咪咕在线人数统计请求
	miguUserOnlineReq struct {
		// 直播Id，创建直播响应的channelId
		Channel string `json:"channel"`
		// 查询时刻
		Time int64 `json:"time,omitempty"`

		BeginTime int64 `json:"beginTime,omitempty"`

		EndTime int64 `json:"endTime,omitempty"`
		// 平台 VOD或者LIVE
		Platform string `json:"platform,omitempty"`
		// 时间粒度
		Type int `json:"type,omitempty"`
	}

	// 咪咕在线人数统计返回
	miguUserOnlineRsp struct {
		miguBaseRsp

		Result struct {
			Content []struct {
				Channel string `json:"channel"`
				Datas   []struct {
					Num  int64 `json:"num"`
					Time int64 `json:"time"`
				} `json:"datas"`
			} `json:"content"`
		} `json:"result"`
	}
)
