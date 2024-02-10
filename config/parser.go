package config

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"

	"os"

	"github.com/hiddify/ray2sing/ray2sing"
	"github.com/sagernet/sing-box/experimental/libbox"
	"github.com/sagernet/sing-box/option"
	SJ "github.com/sagernet/sing/common/json"
	"github.com/xmdhs/clash2singbox/convert"
	"github.com/xmdhs/clash2singbox/model/clash"
	"gopkg.in/yaml.v3"
)

//go:embed config.json.template
var configByte []byte

func ParseConfig(path string, debug bool) ([]byte, error) {
	content, err := os.ReadFile(path)
	os.Chdir(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	var jsonObj map[string]interface{}

	fmt.Printf("Convert using json\n")
	jsonDecoder := json.NewDecoder(SJ.NewCommentFilter(bytes.NewReader(content)))
	if err := jsonDecoder.Decode(&jsonObj); err == nil {
		if jsonObj["outbounds"] == nil {
			if jsonArray, ok := jsonObj.([]map[string]interface{}); ok {
				jsonObj = map[string]interface{}{"outbounds": jsonArray}
				if jsonArray[0]["type"] == nil {
					return nil, fmt.Errorf("[SingboxParser] no outbounds found")
				}
			} else if jsonObj["type"] == nil {
				return nil, fmt.Errorf("[SingboxParser] no outbounds found")
			} else {
				jsonObj = map[string]interface{}{"outbounds": []interface{}{jsonObj}}
			}
		}

		jsonObj = map[string]interface{}{
			"outbounds": jsonObj["outbounds"],
		}

		newContent, _ := json.MarshalIndent(jsonObj, "", "  ")

		return patchConfig([]byte(newContent), "SingboxParser")
	}
	fmt.Printf("Convert using v2ray\n")
	v2rayStr, err := ray2sing.Ray2Singbox(string(content))
	if err == nil {
		return patchConfig([]byte(v2rayStr), "V2rayParser")
	}
	fmt.Printf("Convert using clash\n")
	clashObj := clash.Clash{}
	if err := yaml.Unmarshal(content, &clashObj); err == nil && clashObj.Proxies != nil {
		if len(clashObj.Proxies) == 0 {
			return nil, fmt.Errorf("[ClashParser] no outbounds found")
		}
		converted, err := convert.Clash2sing(clashObj)
		if err != nil {
			return nil, fmt.Errorf("[ClashParser] converting clash to sing-box error: %w", err)
		}
		output := configByte
		output, err = convert.Patch(output, converted, "", "", nil)
		if err != nil {
			return nil, fmt.Errorf("[ClashParser] patching clash config error: %w", err)
		}
		return patchConfig(output, "ClashParser")
	}

	return nil, fmt.Errorf("unable to determine config format")
}

func patchConfig(content []byte, name string) ([]byte, error) {
	options := option.Options{}
	err := json.Unmarshal(content, &options)
	if err != nil {
		return nil, fmt.Errorf("[SingboxParser] unmarshal error: %w", err)
	}

	for i, base := range options.Outbounds {
		err := patchWarp(&base)
		if err != nil {
			return nil, fmt.Errorf("[Warp] patch warp error: %w", err)
		}
		options.Outbounds[i] = base

	}

	content, _ = json.MarshalIndent(options, "", "  ")
	fmt.Printf("%s\n", content)
	return validateResult(content, name)
}

func validateResult(content []byte, name string) ([]byte, error) {

	err := libbox.CheckConfig(string(content))
	if err != nil {
		return nil, fmt.Errorf("[%s] invalid sing-box config: %w", name, err)
	}
	return content, nil
}
