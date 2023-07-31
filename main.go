package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ruff.io/coald/pkg/log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	DefaultQos = 1

	host = "localhost"
	port = 1883
)

var (
	thingId  = "example"
	password = "example"

	state = map[string]any{
		"csq":   0,
		"power": false,
	}
)

var mqttClient mqtt.Client

func main() {
	start()
	select {}
}

func start() {
	retryConn()
}
func subscribe() {
	// Subscribe shadow topics and direct method invoke topic
	receiveShadowUpdateResp()
	receiveDirectMethodInvoke()

	// Report the current state of light when it boot, then server can get it's state by query Shadow
	// updateShadowReported(state)
}

func retryConn() {
	for {
		if err := connectTioByMqtt(); err != nil {
			time.Sleep(time.Second * 10)
		} else {
			break
		}
	}
}

func connectTioByMqtt() error {
	brk := fmt.Sprintf("tcp://%s:%d", host, port)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(brk)
	opts.SetClientID(thingId)
	opts.SetUsername(thingId)
	opts.SetPassword(password)

	opts.OnConnect = func(c mqtt.Client) {
		log.Info("Mqtt connected")
		subscribe()
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		log.Warnf("Mqtt connection lost: %v", err)
	}
	mqttClient = mqtt.NewClient(opts)
	tk := mqttClient.Connect()
	tk.Wait()
	if tk.Error() != nil {
		log.Errorf("connect mqtt to %s error: %v", brk, tk.Error())
		return tk.Error()
	}
	return nil
}

func receiveShadowUpdateResp() error {
	log.Info("[Receive Shadow Update Response] subscribe")

	topicReq := fmt.Sprintf("$iothub/things/%s/shadows/name/default/update/+", thingId)
	accepted := "accepted"
	rejected := "rejected"

	tk := mqttClient.Subscribe(topicReq, 0, func(c mqtt.Client, m mqtt.Message) {
		go func() {
			var acceptedResp StateAcceptedResp
			var rejectedResp ErrResp

			// Server accepted shadow get request
			if strings.HasSuffix(m.Topic(), accepted) {
				_ = json.Unmarshal(m.Payload(), &acceptedResp)
				log.Infof("[Receive Shadow Update Response] update accepted: \n%s", toJsonStr(acceptedResp))
			}

			if strings.HasSuffix(m.Topic(), rejected) {
				_ = json.Unmarshal(m.Payload(), &rejectedResp)
				log.Errorf("[Receive Shadow Update Response] update rejected, code: %d , msg: %s",
					rejectedResp.Code, rejectedResp.Message)
				// Do something when update shadow rejected by code of the response, eg: try agin
				// ...
			}

		}()
	})
	tk.Wait()

	if tk.Error() != nil {
		log.Errorf("mqtt subscribe error: %v", tk.Error())
		return tk.Error()
	}
	return nil
}

// updateShadowReported Report device state by update `Shadow desired`
func updateShadowReported(payload map[string]any) {
	log.Infof("[LightState] Report shadow desired: power: %s, brightness: %v",
		state["power"], state["brightness"])

	r := StateReq{
		ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixMicro()),
		State:       StateDR{Reported: payload},
	}
	reqJson, _ := json.Marshal(r)
	log.Infof("[Set Shadow Reported] \n%s", toJsonStr(r))
	topic := fmt.Sprintf("$iothub/things/%s/shadows/name/default/update", thingId)
	mqttClient.Publish(topic, DefaultQos, false, reqJson)
}

// receiveDirectMethodInvoke
//  1. subscribe the method request topic
//  2. do the method action when receive method request
//  3. send response like a http response
func receiveDirectMethodInvoke() error {
	log.Infof("[Receive Method Request] subscribe method request")

	topicReq := fmt.Sprintf("$iothub/things/%s/methods/+/req", thingId)

	tk := mqttClient.Subscribe(topicReq, 0, func(c mqtt.Client, m mqtt.Message) {
		go func() {
			var req MethodReq
			var resp MethodResp
			arr := strings.Split(m.Topic(), "/")
			method := arr[4]
			log.Infof("[Receive Method Request] method=%s \n%s", method, m.Payload())

			err := json.Unmarshal(m.Payload(), &req)
			if err == nil {
				if m, ok := req.Data.(map[string]any); ok {
					resp = doMethod(method, m)
					resp.ClientToken = req.ClientToken
				}
			} else {
				log.Errorf("[Receive Method Request] device unable to unmarshal method request body %s", m.Payload())
				resp = MethodResp{
					ClientToken: req.ClientToken,
					Data:        nil,
					Message:     fmt.Sprintf("wrong request body: %s", err),
					Code:        400,
				}
			}

			b, _ := json.Marshal(resp)

			topicResp := fmt.Sprintf("$iothub/things/%s/methods/%s/resp", thingId, method)
			mqttClient.Publish(topicResp, 0, false, b)
		}()
	})

	tk.Wait()
	if tk.Error() != nil {
		log.Errorf("mqtt subscribe error: %v", tk.Error())
		return tk.Error()
	}
	return nil
}

const methodHelp = `
app methods:
	lightOn         	开灯
	lightOff        	关灯
`

func doMethod(method string, data map[string]any) MethodResp {
	switch method {
	case "help":
		return MethodResp{Code: 200, Message: "OK", Data: methodHelp}
	case "lightOn":
		return doLightOn()
	case "lightOff":
		return doLightOff()
	default:
		return MethodResp{Code: 400, Message: fmt.Sprintf("unknown method %v", method)}
	}
}

func doLightOn() MethodResp {
	log.Debug("doLightOn")
	return MethodResp{Code: 200, Message: "OK"}
}

func doLightOff() MethodResp {
	log.Debug("doLightOff")
	return MethodResp{Code: 200, Message: "OK"}
}

func toJsonStr(v any) string {
	s, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("Marshal value %v error %v", v, err)
	}
	return string(s)
}
