package ula

import (
	`encoding/json`
	`fmt`
	`strconv`
	`strings`
	`sync`
	`time`

	`github.com/go-resty/resty/v2`
	`github.com/storezhang/gox`
)

var (
	_ Ula         = (*andLive)(nil)
	_ ulaInternal = (*andLive)(nil)
)

// andLive 和直播
type andLive struct {
	resty    *resty.Request
	template ulaTemplate

	tokenCache  sync.Map
	getCache    sync.Map
	recordCache sync.Map
}

// NewAndLive 创建和直播
func NewAndLive(resty *resty.Request) (live *andLive) {
	live = &andLive{
		resty: resty,

		tokenCache:  sync.Map{},
		getCache:    sync.Map{},
		recordCache: sync.Map{},
	}
	live.template = ulaTemplate{andLive: live}

	return
}

func (a *andLive) CreateLive(req *CreateLiveReq, opts ...Option) (id string, err error) {
	return a.template.CreateLive(req, opts...)
}

func (a *andLive) GetPushUrls(id string, opts ...Option) (urls []Url, err error) {
	return a.template.GetPushUrls(id, opts...)
}

func (a *andLive) GetPullCameras(id string, opts ...Option) (cameras []Camera, err error) {
	return a.template.GetPullCameras(id, opts...)
}

func (a *andLive) Stop(id string, opts ...Option) (success bool, err error) {
	return a.template.Stop(id, opts...)
}

func (a *andLive) createLive(req *CreateLiveReq, options *options) (id string, err error) {
	var (
		rsp    = new(andLiveCreateRsp)
		token  string
		rawRsp *resty.Response
	)

	if token, err = a.getToken(options); nil != err {
		return
	}

	url := fmt.Sprintf("%s/api/v10/events/create.json", options.endpoint)
	if rawRsp, err = a.resty.SetFormData(map[string]string{
		"client_id":    options.andLive.clientId,
		"access_token": token,
		"name":         req.Title,
		"starttime":    req.StartTime.Format(),
		"endTime":      req.EndTime.Format(),
		"uid":          options.andLive.uid,
		"save_video":   "1",
	}).Post(url); nil != err {
		return
	}

	if err = json.Unmarshal(rawRsp.Body(), rsp); nil != err {
		return
	}

	if 0 != rsp.ErrCode {
		err = gox.NewCodeError(gox.ErrorCode(rsp.ErrCode), rsp.ErrMsg, nil)
	} else {
		id = strconv.FormatInt(rsp.Id, 10)
	}

	return
}

func (a *andLive) getPushUrls(id string, options *options) (urls []Url, err error) {
	var rsp *andLiveGetRsp
	if rsp, err = a.get(id, options, true); nil != err {
		return
	}

	urls = []Url{{
		Type: VideoFormatTypeRtmp,
		Link: rsp.PushUrl,
	}}

	return
}

func (a *andLive) getPullCameras(id string, options *options) (cameras []Camera, err error) {
	var rsp *andLiveGetRsp
	if rsp, err = a.get(id, options, true); nil != err {
		return
	}

	if 0 != rsp.ErrCode {
		err = gox.NewCodeError(gox.ErrorCode(rsp.ErrCode), rsp.ErrMsg, nil)
	} else {
		var urls []string
		// 如果直播还没有结束，应该返回拉流地址
		if rsp.EndTime.Time().After(time.Now()) {
			// 取得和直播返回的直播编号，这里做特殊处理，查看返回可以发现规律
			// 20210601210100_7HMMZ6X4
			// http://wshls.live.migucloud.com/live/7HMMZ6X4_C0/playlist.m3u8
			// rtmp://devlivepush.migucloud.com/live/7HMMZ6X4_C0
			urls = []string{fmt.Sprintf("https://wshlslive.migucloud.com/live/%s_C0/playlist.m3u8", rsp.miguId())}
		} else {
			if urls, err = a.recordUrls(id, options); nil != err {
				return
			}
		}
		cameras = []Camera{{
			Index: 1,
			Videos: []Video{{
				Type: VideoTypeOriginal,
				Urls: a.parseLinks(urls),
			}},
		}}
	}

	return
}

func (a *andLive) stop(id string, options *options) (success bool, err error) {
	var (
		rsp    = new(andLiveStopRsp)
		token  string
		rawRsp *resty.Response
	)

	if token, err = a.getToken(options); nil != err {
		return
	}

	url := fmt.Sprintf("%s/api/v10/events/stop.json", options.endpoint)
	if rawRsp, err = a.resty.SetFormData(map[string]string{
		"client_id":    options.andLive.clientId,
		"access_token": token,
		"id":           id,
	}).Post(url); nil != err {
		return
	}

	if err = json.Unmarshal(rawRsp.Body(), rsp); nil != err {
		return
	}
	success = 0 == rsp.ErrCode && rsp.Success

	return
}

func (a *andLive) recordUrls(id string, options *options) (urls []string, err error) {
	var (
		cache interface{}
		ok    bool
		rsp   *andLiveGetRsp
	)

	key := id
	if cache, ok = a.recordCache.Load(key); ok {
		urls = cache.([]string)
	} else {
		if rsp, err = a.get(id, options, false); nil != err {
			return
		} /**/
		for _, url := range rsp.Urls {
			urls = append(urls, strings.ReplaceAll(url, "http://mgcdn.vod.migucloud.com", "https://mgcdnvod.migucloud.com"))
		}
		a.recordCache.Store(key, urls)
	}

	return
}

func (a *andLive) get(id string, options *options, useCache bool) (rsp *andLiveGetRsp, err error) {
	var (
		cache  interface{}
		ok     bool
		token  string
		rawRsp *resty.Response
	)

	if !useCache {
		a.getCache.Delete(id)
	}

	key := id
	if cache, ok = a.getCache.Load(key); ok {
		rsp = cache.(*andLiveGetRsp)
	}
	if nil != rsp {
		return
	}

	if token, err = a.getToken(options); nil != err {
		return
	}

	url := fmt.Sprintf("%s/api/v10/events/get.json", options.endpoint)
	if rawRsp, err = a.resty.SetQueryParams(map[string]string{
		"client_id":    options.andLive.clientId,
		"access_token": token,
		"id":           id,
	}).Get(url); nil != err {
		return
	}

	rsp = new(andLiveGetRsp)
	if err = json.Unmarshal(rawRsp.Body(), rsp); nil != err {
		return
	}
	a.getCache.Store(key, rsp)

	return
}

func (a *andLive) getToken(options *options) (token string, err error) {
	var (
		cache  interface{}
		ok     bool
		rawRsp *resty.Response
		rsp    = new(getAndLiveTokenRsp)
	)

	key := options.andLive.clientId
	if cache, ok = a.tokenCache.Load(key); ok {
		var validate bool
		if token, validate = cache.(*andLiveToken).validate(); validate {
			return
		} else {
			a.tokenCache.Delete(key)
		}
	}

	url := fmt.Sprintf("%s/auth/oauth2/access_token", options.endpoint)
	if rawRsp, err = a.resty.SetFormData(map[string]string{
		"client_id":     options.andLive.clientId,
		"client_secret": options.andLive.clientSecret,
		"grant_type":    "client_credentials",
	}).Post(url); nil != err {
		return
	}
	if err = json.Unmarshal(rawRsp.Body(), rsp); nil != err {
		return
	}

	if 1001 == rsp.ErrCode {
		err = &gox.CodeError{Message: rsp.ErrMsg}
	} else {
		token = rsp.AccessToken
		a.tokenCache.Store(key, &andLiveToken{
			accessToken: token,
			expiresIn:   time.Now().Add(time.Duration(1000 * rsp.ExpiresIn)),
		})
	}

	return
}

func (a *andLive) parseLinks(links []string) (urls []Url) {
	urls = make([]Url, 0, len(links))
	for _, link := range links {
		urls = append(urls, Url{
			Type: VideoFormatTypeHls,
			Link: link,
		})
	}

	return
}
