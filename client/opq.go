package client

import (
	"encoding/base64"
	"errors"
	"fmt"
	"lit-life-bot/config"
	"lit-life-bot/model"
	"log"

	"github.com/icepie/oh-my-lit/client/sec"
	"github.com/icepie/oh-my-lit/client/zhyd"
	"github.com/mcoo/OPQBot"
	"github.com/mcoo/OPQBot/session"
)

var (
	OPQ OPQBot.BotManager
)

// 用户绑定智慧门户
func UserBindLitSec(s session.Session, packet *OPQBot.FriendMsgPack) error {

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

// 用户使用 智慧用电相关
func UserZhyd(user model.User, s session.Session, packet *OPQBot.FriendMsgPack) {

	zhydUser, err := zhyd.NewZhydUser(user.StuID, user.SecPassword)
	if err != nil {
		log.Println("实例化用户失败: ", err)
		OPQ.Send(OPQBot.SendMsgPack{
			SendToType: OPQBot.SendToTypeFriend,
			ToUserUid:  packet.FromUin,
			Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
		})
		return
	}

	b, err := zhydUser.IsNeedCaptcha()
	if err != nil {
		log.Println("获取用户信息失败: ", err)
		OPQ.Send(OPQBot.SendMsgPack{
			SendToType: OPQBot.SendToTypeFriend,
			ToUserUid:  packet.FromUin,
			Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
		})
		return
	}

	if b {
		// log.Println("需要验证码")
		// pix, err := zhydUser.GetCaptche()
		// if err != nil {
		log.Println("获取验证码失败: ", err)
		OPQ.Send(OPQBot.SendMsgPack{
			SendToType: OPQBot.SendToTypeFriend,
			ToUserUid:  packet.FromUin,
			Content:    OPQBot.SendTypeTextMsgContent{Content: "未知错误(请及时反馈)"},
		})
		return

	} else {
		err = zhydUser.Login()
		if err != nil {
			log.Println("获取验证码状态失败: ", err)
			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "未知错误(请及时反馈)"},
			})
			return
		}

		isLogged, err := zhydUser.IsLogged()
		if isLogged {

			if packet.Content == "/宿舍用电" {
				de, err := zhydUser.GetDormElectricity()
				if err != nil {
					log.Println("获取余电额度失败: ", err)
					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
					})
					return
				}

				deStr := fmt.Sprintf("宿舍用电: \n\t\t宿舍楼: %s \n\t\t房间: %s \n\t\t余电: %v 度 \n\t\t余额: %v 元", de.BuildName, de.Room, de.Electricity, de.Balance)

				// todo

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: deStr},
				})

				return
			} else if packet.Content == "/历史用电" {

				ed, err := zhydUser.GetElectricityDetails()
				if err != nil {
					log.Println("获取历史用电失败: ", err)
					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
					})
				}

				edStr := "历史用电:\n"

				for _, v := range ed.Details {
					edStr += "\t" + v.Time.Format("2006-01-02") + ": " + fmt.Sprint(v.Value) + " 度\n"
				}

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: edStr},
				})

				return

			} else if packet.Content == "/充电记录" {

				// log.Println(ed)

				cr, err := zhydUser.GetChargeRecords()
				if err != nil {
					log.Println("获取充值记录失败: ", err)
					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
					})
					return
				}

				crStr := "充电记录:\n"

				for _, v := range cr.Mx {
					crStr += "\t" + v.Accounttime.Time.Format("2006-01-02") + ": " + v.Inmoney + "元\n"
				}

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: crStr},
				})

				return

			} else {

				log.Println("似乎未登陆: ", err)
				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "未知错误(请及时反馈)"},
				})

				return
			}
		}
	}
}

// 处理群消息
func handleGroupMsg(botQQ int64, packet *OPQBot.GroupMsgPack) {
	packet.Next(botQQ, packet)
}

// 处理私聊消息
func handleFriendMsg(botQQ int64, packet *OPQBot.FriendMsgPack) {

	// 排除机器人自己
	if packet.FromUin == botQQ {
		return
	}

	s := OPQ.Session.SessionStart(packet.FromUin)

	status, _ := s.GetString("status")
	stopTime, _ := s.GetInt("stop_time")

	// 查找用户
	var user model.User
	err := model.DB.Where("qq = ?", packet.FromUin).First(&user).Error
	if err != nil {
		log.Println(err)
	}

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

	if packet.Content == "/菜单" {

		content := "基础功能: \n\t/绑定 - 绑定智慧门户帐号 \n\t/状态 - 查看绑定状态 \n\t/解绑 - 彻底解除绑定 \n\n智慧控电: \n\t/宿舍用电 - 查询当前宿舍剩余用电 \n\t/历史用电 - 查询宿舍历史用电情况 \n\t /充电记录 - 查询宿舍电量充值记录"

		OPQ.Send(OPQBot.SendMsgPack{
			SendToType: OPQBot.SendToTypeFriend,
			ToUserUid:  packet.FromUin,
			Content:    OPQBot.SendTypeTextMsgContent{Content: content},
		})

		return
	}

	// 绑定状态
	if status == "binding" {
		UserBindLitSec(s, packet)
	}

	// 判断用户状态
	if status == "" {

		if packet.Content == "/绑定" {
			if user.QQ == 0 {
				s.Set("status", "binding")
				s.Set("step", "init")
				err := UserBindLitSec(s, packet)
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
					Content:    OPQBot.SendTypeTextMsgContent{Content: "您已绑定学号: " + user.StuID + " ! \n\n如需解除绑定, 请输入 /解绑 !"},
				})
			}
			return
		}

		// 未绑定的情况
		if user.QQ == 0 {

			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "您未绑定任何学号! \n\n如需进行绑定操作, 请输入 /绑定 !"},
			})

			return

		} else {
			if packet.Content == "/解绑" {

				err := model.DB.Delete(&user).Error
				if err != nil {
					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
					})
					return
				}

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "解除绑定成功! 洛洛会想你的 :( "},
				})

			} else if packet.Content == "/状态" {

				content := fmt.Sprintf("姓名: %s \n学号: %s \n智慧门户: 已绑定", user.Name, user.StuID)

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: content},
				})

			} else if packet.Content == "/宿舍用电" || packet.Content == "/历史用电" || packet.Content == "/充电记录" {
				UserZhyd(user, s, packet)
			}
		}
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
