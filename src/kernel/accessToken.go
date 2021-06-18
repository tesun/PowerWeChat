package kernel

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	http2 "github.com/ArtisanCloud/go-libs/http"
	httpContract "github.com/ArtisanCloud/go-libs/http/contract"
	"github.com/ArtisanCloud/go-libs/object"
	"github.com/ArtisanCloud/go-wechat/src/kernel/contract"
	response2 "github.com/ArtisanCloud/go-wechat/src/kernel/response"
	"net/http"
	"time"
)

type AccessToken struct {
	App *ApplicationInterface

	*http2.HttpRequest
	*http2.HttpResponse

	RequestMethod      string
	EndpointToGetToken string
	QueryName          string
	Token              object.HashMap
	TokenKey           string
	CachePrefix        string

	*CacheToken

	GetCredentials func() *object.StringMap
}

func NewAccessToken(app *ApplicationInterface) *AccessToken {
	config := (*app).GetContainer().GetConfig()

	token := &AccessToken{
		App:         app,
		HttpRequest: http2.NewHttpRequest(config),

		RequestMethod:      "GET",
		EndpointToGetToken: "",
		QueryName:          "",
		Token:              nil,
		TokenKey:           "access_token",
		CachePrefix:        "ac.go.wechat.kernel.access_token.",

		CacheToken: &CacheToken{},
	}

	return token
}

func (accessToken *AccessToken) GetRefreshedToken() (*response2.ResponseGetToken, error) {
	return accessToken.GetToken(true)
}

func (accessToken *AccessToken) GetToken(refresh bool) (resToken *response2.ResponseGetToken, err error) {
	cacheKey := accessToken.getCacheKey()
	cache := accessToken.getCache()

	// get token from cache
	if !refresh && cache.Has(cacheKey) {
		value, err := cache.Get(cacheKey, nil)
		if err == nil {
			token := value.(*object.HashMap)
			return &response2.ResponseGetToken{
				AccessToken: (*token)[accessToken.TokenKey].(string),
				ExpiresIn:   (*token)["expires_in"].(int),
			}, err
		}
	}

	// request token from wx
	response, err := accessToken.requestToken(accessToken.GetCredentials())
	if err != nil {
		return nil, err
	}

	// save token into cache
	resToken = response.(*response2.ResponseGetToken)
	var expireIn int = 7200
	if resToken.ExpiresIn > 0 {
		expireIn = resToken.ExpiresIn
	}
	accessToken.SetToken(resToken.AccessToken, expireIn)

	// tbd dispatch a event for AccessTokenRefresh

	return resToken, err
}

func (accessToken *AccessToken) SetToken(token string, lifeTime int) (tokenInterface contract.AccessTokenInterface, err error) {
	if lifeTime <= 0 {
		lifeTime = 7200
	}

	// set token into cache
	cache := accessToken.getCache()
	err = cache.Set(accessToken.getCacheKey(), &object.HashMap{
		accessToken.TokenKey: token,
		"expires_in":         lifeTime,
	}, time.Duration(lifeTime)*time.Second)

	if !cache.Has(accessToken.getCacheKey()) {
		return nil, errors.New("failed to cache access token")
	}
	return accessToken, err

}

func (accessToken *AccessToken) Refresh() contract.AccessTokenInterface {

	return nil
}

func (accessToken *AccessToken) requestToken(credentials *object.StringMap) (httpContract.ResponseContract, error) {

	// tbf
	//return &response2.ResponseGetToken{
	//	AccessToken: "5Vp9FFQufiOxNjf9XdsMIvpkjft3VaQadnqwiiTQHBA5AqpJMUuyxaC1GHMaZrN9LAVvcfex_IIz_NEqC2JFtPJumsXHA7IVgtlWnG2j5tD3Iqt70uR2njTXHFrbU4qu-JceHA9RCl1QP7B46kbtRjC_h-fPQjAeHo8QMzfFGf9dZ6AdgD72QbhZB8VOIGikSCOjsho0ayk5sgcGDM-IDg",
	//	ExpiresIn:   7200,
	//	ResponseWX: &response2.ResponseWX{
	//		ErrCode: 0,
	//		ErrMSG:  "ok",
	//	},
	//}, nil

	res, err := accessToken.sendRequest(credentials)
	if err != nil {
		return nil, err
	}
	token := res.(*response2.ResponseGetToken)

	if token == nil || token.AccessToken == "" {
		return nil, errors.New(fmt.Sprintf("Request access_token fail: %v", res))
	}

	return token, nil
}

func (accessToken *AccessToken) ApplyToRequest(request *http.Request, requestOptions *object.HashMap) (*http.Request, error) {

	// query Access Token map
	mapToken, err := accessToken.getQuery()
	if err != nil {
		return nil, err
	}
	q := request.URL.Query()
	for key, value := range *mapToken {
		q.Set(key, value)
	}
	request.URL.RawQuery = q.Encode()

	return request, err
}

func (accessToken *AccessToken) sendRequest(credential *object.StringMap) (httpContract.ResponseContract, error) {

	key := "json"
	if accessToken.RequestMethod == "GET" {
		key = "query"
	}
	options := &object.HashMap{
		key: credential,
	}

	res := &response2.ResponseGetToken{}

	strEndpoint, err := accessToken.getEndpoint()
	if err != nil {
		return nil, err
	}
	accessToken.SetHttpClient(accessToken.GetHttpClient()).PerformRequest(
		strEndpoint,
		accessToken.RequestMethod,
		options,
		res,
	)
	return res, nil
}

func (accessToken *AccessToken) getCacheKey() string {
	data, _ := json.Marshal(accessToken.GetCredentials())
	buffer := md5.Sum(data)

	return accessToken.CachePrefix + string(buffer[:])
}

func (accessToken *AccessToken) getQuery() (*object.StringMap, error) {
	// set the current token key
	var key string
	if accessToken.QueryName != "" {
		key = accessToken.QueryName
	} else {
		key = accessToken.TokenKey
	}

	// get token string map
	resToken, err := accessToken.GetToken(false)
	if err != nil {
		return nil, err
	}
	arrayReturn := &object.StringMap{
		key: resToken.AccessToken,
	}

	return arrayReturn, err

}

func (accessToken *AccessToken) getEndpoint() (string, error) {
	if accessToken.EndpointToGetToken == "" {
		return "", errors.New("no endpoint for access token request")
	}

	return accessToken.EndpointToGetToken, nil
}

func (accessToken *AccessToken) getTokenKey() string {
	return accessToken.TokenKey
}
