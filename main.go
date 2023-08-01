package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ruff.io/coald/pkg/log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	DefaultQos = 1
)

type Opts struct {
	host     string
	port     int
	thingId  string
	password string

	thingIdOfCtrl string
	isCtrlNode    bool
	isExecNode    bool
}

var (
	opts Opts
)

var mqttClient mqtt.Client

func init() {
	flag.StringVar(&opts.host, "h", "", "MQTT服务地址，域名或IP")
	flag.IntVar(&opts.port, "p", 1883, "MQTT 服务端口")
	flag.StringVar(&opts.thingId, "i", "", "当前设备的 thingId")
	flag.StringVar(&opts.password, "s", "", "当前设备的接入密码")
	flag.StringVar(&opts.thingIdOfCtrl, "ci", "", "控制端的设备 thingId, 作为受控端时必需提供该值")
	flag.BoolVar(&opts.isCtrlNode, "c", false, "是否为控制端")
	flag.BoolVar(&opts.isExecNode, "e", false, "是否为受控执行端")
	flag.Parse()
	if opts.host == "" || opts.thingId == "" || opts.password == "" {
		log.Fatal("host, thingId and password must be provided")
	}
	if opts.isExecNode && opts.thingIdOfCtrl == "" {
		log.Fatal("must provide thingId of controller when this node is execute node")
	}

	log.Infof("option: %#v", opts)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if sig := signalHandler(ctx); sig != nil {
			cancel()
			log.Info(fmt.Sprintf("shutdown by signal: %s", sig))
		}
	}()

	start(ctx)

	<-ctx.Done()
	time.Sleep(time.Second) // wait for clean
}

func start(ctx context.Context) {
	retryConnMqtt(ctx)
	go syncStatusLoop(ctx)
	if opts.isCtrlNode {
		go m2mCtlLoop(ctx, opts.thingId, mqttClient)
	}
}
func subscribe() {
	// Subscribe shadow topics and direct method invoke topic
	receiveShadowUpdateResp()
	receiveDirectMethodInvoke()
	if opts.isExecNode {
		subscribeM2M(mqttClient, opts.thingIdOfCtrl)
	}
}

func retryConnMqtt(ctx context.Context) {
	for {
		if err := connectTioByMqtt(opts); err != nil {
			if ctx.Err() != nil {
				return
			}
			time.Sleep(time.Second * 10)
		} else {
			return
		}
	}
}

func connectTioByMqtt(opt Opts) error {
	brk := fmt.Sprintf("tcp://%s:%d", opt.host, opt.port)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(brk)
	opts.SetClientID(opt.thingId)
	opts.SetUsername(opt.thingId)
	opts.SetPassword(opt.password)

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

	topicReq := fmt.Sprintf("$iothub/things/%s/shadows/name/default/update/+", opts.thingId)
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
	log.Infof("[Set Shadow Reported] raw: %v", payload)

	r := StateReq{
		ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixMicro()),
		State:       StateDR{Reported: payload},
	}
	reqJson, _ := json.Marshal(r)
	topic := fmt.Sprintf("$iothub/things/%s/shadows/name/default/update", opts.thingId)
	tk := mqttClient.Publish(topic, DefaultQos, false, reqJson)
	tk.Wait()
	if tk.Error() != nil {
		log.Errorf("[Set Shadow Reported] mqtt publish error: %s", tk.Error())
	}
}

// receiveDirectMethodInvoke
//  1. subscribe the method request topic
//  2. do the method action when receive method request
//  3. send response like a http response
func receiveDirectMethodInvoke() error {
	log.Infof("[Receive Method Request] subscribe method request")

	topicReq := fmt.Sprintf("$iothub/things/%s/methods/+/req", opts.thingId)

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

			topicResp := fmt.Sprintf("$iothub/things/%s/methods/%s/resp", opts.thingId, method)
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
	lightOn(true)
	return MethodResp{Code: 200, Message: "OK"}
}

func doLightOff() MethodResp {
	log.Debug("doLightOff")
	lightOn(false)
	return MethodResp{Code: 200, Message: "OK"}
}

func syncStatusLoop(ctx context.Context) {
	ch := make(chan map[string]any)
	go onUbusEvent(ctx, ubusStatusEventPath, ch)
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-ch:
			updateShadowReported(map[string]any{
				"status": e,
			})
		}
	}
}

func toJsonStr(v any) string {
	s, err := json.Marshal(v)
	if err != nil {
		log.Fatalf("Marshal value %v error %v", v, err)
	}
	return string(s)
}

func signalHandler(ctx context.Context) error {
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGABRT)
	select {
	case sig := <-c:
		return fmt.Errorf("%s", sig)
	case <-ctx.Done():
		return nil
	}
}
