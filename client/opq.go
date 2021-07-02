package client

import (
	"encoding/base64"
	"errors"
	"lit-life-bot/config"
	"lit-life-bot/model"
	"log"

	"github.com/icepie/oh-my-lit/client/sec"
	"github.com/mcoo/OPQBot"
	"github.com/mcoo/OPQBot/session"
)

var (
	OPQ OPQBot.BotManager
)

// 用户绑定智慧门户
func bindLitSec(s session.Session, packet *OPQBot.FriendMsgPack) error {

	step, _ := s.GetString("step")

	if step == "init" {

		OPQ.Send(OPQBot.SendMsgPack{
			SendToType: OPQBot.SendToTypeFriend,
			ToUserUid:  packet.FromUin,
			Content:    OPQBot.SendTypeTextMsgContent{Content: "请您的输入学号!"},
		})

		s.Set("step", "username")

	} else if step == "username" {

		// 检查长度
		if len(packet.Content) != 9 {

			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "学号格式有误, 请检查后重新输入!"},
			})

			stopTime, _ := s.GetInt("stop_time")
			s.Set("stop_time", stopTime+1)

			return nil
		}

		// 查找用户
		count := 0
		model.DB.First(&model.User{}).Where("stu_id = ?", packet.Content).Count(&count)

		if count > 0 {
			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "该学号已被其他QQ绑定! 请解绑后再进行绑定!"},
			})

			return errors.New("fail to bind")
		}

		s.Set("username", packet.Content)

		OPQ.Send(OPQBot.SendMsgPack{
			SendToType: OPQBot.SendToTypeFriend,
			ToUserUid:  packet.FromUin,
			Content:    OPQBot.SendTypeTextMsgContent{Content: "请输入您的智慧门户密码!"},
		})

		s.Set("step", "password")

	} else if step == "password" {

		username, _ := s.GetString("username")

		s.Set("password", packet.Content)

		secUser, err := sec.NewSecUser(username, packet.Content)
		if err != nil {
			log.Println("实例化用户失败: ", err)
			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
			})

			return err
		}

		s.Set("sec_user", secUser)

		isNeedcap, err := secUser.IsNeedCaptcha()
		if err != nil {
			log.Println("获取状态信息失败: ", err)
			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
			})

			return err
		}

		if isNeedcap {
			log.Println("需要验证码")

			pix, err := secUser.GetCaptche()

			if err != nil {
				log.Println("获取验证码失败: ", err)
				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
				})

				return err
			}

			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypePicMsgByBase64Content{Content: "请输入验证码!", Base64: base64.StdEncoding.EncodeToString(pix)},
			})

			s.Set("step", "cap")

		} else {
			log.Println("不需要验证码")
			err = secUser.Login()
			if err != nil {
				log.Println("登陆失败: ", err)

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "绑定失败: " + err.Error()},
				})

				return err
			}

			if secUser.IsLogged() {
				// 进行门户登陆
				secUser.PortalLogin()

				if secUser.IsPortalLogged() {

					test, err := secUser.GetCurrentMember()
					if err != nil {
						log.Fatal("获取个人信息失败: ", err)

						OPQ.Send(OPQBot.SendMsgPack{
							SendToType: OPQBot.SendToTypeFriend,
							ToUserUid:  packet.FromUin,
							Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
						})

						return err
					}

					// 准备创建用户
					user := model.User{
						QQ:          packet.FromUin,
						StuID:       secUser.Username,
						SecPassword: secUser.Password,
						Identity:    model.UserIdentity,
						Name:        test.Obj.MemberNickname,
						Gender:      uint(test.Obj.MemberSex),
					}

					err = model.DB.Create(&user).Error
					if err != nil {
						OPQ.Send(OPQBot.SendMsgPack{
							SendToType: OPQBot.SendToTypeFriend,
							ToUserUid:  packet.FromUin,
							Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
						})

						return err
					}

					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: "恭喜" + test.Obj.MemberNickname + "同学绑定成功!" + "\n快输入 /菜单 进行探索吧!"},
					})

					s.Delete("status")
					s.Delete("step")
					s.Delete("username")
					s.Delete("password")
					s.Delete("sec_user")
					s.Delete("stop_time")

				}
			} else {
				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "未知错误(请及时反馈)!"},
				})

				return errors.New("fail to PortalLogin")
			}

		}
	} else if step == "cap" {
		log.Println("验证码:", packet.Content)

		secUserIf, _ := s.Get("sec_user")

		secUser, ok := secUserIf.(sec.SecUser)
		if !ok {

			return errors.New("fali to get the sec_user value")
		}

		//secUser, err := sec.NewSecUser(username, password)

		err := secUser.LoginWithCap(packet.Content)
		if err != nil {
			log.Println("登陆失败: ", err)
			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "绑定失败: " + err.Error()},
			})

			return err
		}

		if secUser.IsLogged() {
			// 进行门户登陆
			secUser.PortalLogin()

			if secUser.IsPortalLogged() {

				test, err := secUser.GetCurrentMember()
				if err != nil {
					log.Fatal("获取个人信息失败: ", err)

					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
					})

					return err
				}

				// 准备创建用户
				user := model.User{
					QQ:          packet.FromUin,
					StuID:       secUser.Username,
					SecPassword: secUser.Password,
					Identity:    model.UserIdentity,
					Name:        test.Obj.MemberNickname,
					Gender:      uint(test.Obj.MemberSex),
				}

				err = model.DB.Create(&user).Error
				if err != nil {
					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
					})

					return err
				}

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "恭喜" + test.Obj.MemberNickname + "同学绑定成功!" + "\n\n快输入 /菜单 进行探索吧!"},
				})

				s.Delete("status")
				s.Delete("step")
				s.Delete("username")
				s.Delete("password")
				s.Delete("sec_user")
				s.Delete("stop_time")

			}

		} else {

			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "未知错误(请重试, 或及时反馈)!"},
			})

			return errors.New("fail to PortalLogin")
		}

	}

	return nil

}

// 处理群消息
func handleGroupMsg(botQQ int64, packet *OPQBot.GroupMsgPack) {
	packet.Next(botQQ, packet)
}

// 处理私聊消息
func handleFriendMsg(botQQ int64, packet *OPQBot.FriendMsgPack) {

	// 查找用户
	var user model.User
	count := 0
	model.DB.First(&user, packet.FromUin).Count(&count)

	s := OPQ.Session.SessionStart(packet.FromUin)

	status, _ := s.GetString("status")
	stopTime, _ := s.GetInt("stop_time")

	// 无条件暂停
	if (packet.Content == "/停止" && status != "") || stopTime >= 2 {
		s.Delete("status")
		s.Set("stop_time", 0)
		OPQ.Send(OPQBot.SendMsgPack{
			SendToType: OPQBot.SendToTypeFriend,
			ToUserUid:  packet.FromUin,
			Content:    OPQBot.SendTypeTextMsgContent{Content: "注意: 本次操作已意外中断!"},
		})

	}

	// 判断用户状态
	if status == "" {

		if packet.Content == "/绑定" {
			if count == 0 {
				s.Set("status", "binding")
				s.Set("step", "init")
				err := bindLitSec(s, packet)
				if err != nil {
					s.Delete("status")
					s.Delete("step")
					s.Delete("username")
					s.Delete("password")
					s.Delete("sec_user")
					s.Delete("stop_time")
				}
			} else {
				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "您已绑定学号" + user.StuID + "! \n\n 如需解除绑定, 请输入 /解绑!"},
				})
			}

		}
	}

	// 绑定状态
	if status == "binding" {
		bindLitSec(s, packet)
	}

	log.Println(packet.FromUin, " -> ", packet.Content)

	packet.Next(botQQ, packet)

}

func OPQStart() {
	OPQ = OPQBot.NewBotManager(config.ProConf.OPQ.QQ, config.ProConf.OPQ.Url)

	OPQ.SetSendDelayed(1000)
	// 设置最大重试次数
	OPQ.SetMaxRetryCount(5)

	err := OPQ.Start()
	if err != nil {
		log.Println(err.Error())
	}
	defer OPQ.Stop()

	err = OPQ.AddEvent(OPQBot.EventNameOnGroupMessage, handleGroupMsg)
	if err != nil {
		log.Println(err.Error())
	}
	err = OPQ.AddEvent(OPQBot.EventNameOnFriendMessage, handleFriendMsg)
	if err != nil {
		log.Println(err.Error())
	}
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupShut, func(botQQ int64, packet *OPQBot.GroupShutPack) {
		log.Println(botQQ, packet)
	})
	if err != nil {
		log.Println(err.Error())
	}
	err = OPQ.AddEvent(OPQBot.EventNameOnConnected, func() {
		log.Println("连接成功！！！")
	})
	if err != nil {
		log.Println(err.Error())
	}
	err = OPQ.AddEvent(OPQBot.EventNameOnDisconnected, func() {
		log.Println("连接断开！！")
	})
	if err != nil {
		log.Println(err.Error())
	}
	err = OPQ.AddEvent(OPQBot.EventNameOnOther, func(botQQ int64, e interface{}) {
		log.Println(err)
	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupSystemNotify, func(botQQ int64, e *OPQBot.GroupSystemNotifyPack) {
		log.Println(err)
	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupRevoke, func(botQQ int64, e *OPQBot.GroupRevokePack) {
		log.Println(err)
	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupJoin, func(botQQ int64, e *OPQBot.GroupJoinPack) {
		log.Println(err)
	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupAdmin, func(botQQ int64, e *OPQBot.GroupAdminPack) {
		log.Println(err)
	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupExit, func(botQQ int64, e *OPQBot.GroupExitPack) {
		log.Println(err)
	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupExitSuccess, func(botQQ int64, e *OPQBot.GroupExitSuccessPack) {
		log.Println(err)
	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupAdminSysNotify, func(botQQ int64, e *OPQBot.GroupAdminSysNotifyPack) {
		log.Println(err)
	})
	if err != nil {
		log.Println(err.Error())
	}

	OPQ.Wait()
}
