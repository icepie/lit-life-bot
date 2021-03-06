package client

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"lit-life-bot/config"
	"lit-life-bot/model"
	"log"
	"strings"
	"time"

	"github.com/icepie/oh-my-lit/client/sec"
	"github.com/icepie/oh-my-lit/client/zhyd"
	"github.com/mcoo/OPQBot"
	"github.com/mcoo/OPQBot/session"
	"github.com/wcharczuk/go-chart/v2"
)

var (
	OPQ           OPQBot.BotManager
	MemberUinList []int64
)

// 用户绑定智慧门户
func UserBindLitSec(s session.Session, packet *OPQBot.FriendMsgPack) error {

	step, _ := s.GetString("step")

	if step == "init" {

		OPQ.Send(OPQBot.SendMsgPack{
			SendToType: OPQBot.SendToTypeFriend,
			ToUserUid:  packet.FromUin,
			Content:    OPQBot.SendTypeTextMsgContent{Content: "请发送您的学号!"},
		})

		s.Set("step", "username")

	} else if step == "username" {

		// 检查长度
		if len(packet.Content) != 9 {

			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "学号格式有误, 请检查后重新发送!"},
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
			Content:    OPQBot.SendTypeTextMsgContent{Content: "请发送您的智慧门户密码! (请在您信任我时发送, 或发送 /停止 中断绑定操作)"},
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
				Content:    OPQBot.SendTypePicMsgByBase64Content{Content: "请发送验证码!", Base64: base64.StdEncoding.EncodeToString(pix)},
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

				s.Delete("status")
				s.Delete("step")
				s.Delete("username")
				s.Delete("password")
				s.Delete("sec_user")
				s.Delete("stop_time")

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
						Content:    OPQBot.SendTypeTextMsgContent{Content: "恭喜" + test.Obj.MemberNickname + "同学绑定成功!" + "\n快发送 /菜单 进行探索吧!"},
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
					Content:    OPQBot.SendTypeTextMsgContent{Content: "未知错误(请及时反馈或重试)!"},
				})

				s.Delete("status")
				s.Delete("step")
				s.Delete("username")
				s.Delete("password")
				s.Delete("sec_user")
				s.Delete("stop_time")

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

			s.Delete("status")
			s.Delete("step")
			s.Delete("username")
			s.Delete("password")
			s.Delete("sec_user")
			s.Delete("stop_time")

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
					Content:    OPQBot.SendTypeTextMsgContent{Content: "恭喜" + test.Obj.MemberNickname + "同学绑定成功!" + "\n\n快发送 /菜单 进行探索吧!"},
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

			s.Delete("status")
			s.Delete("step")
			s.Delete("username")
			s.Delete("password")
			s.Delete("sec_user")
			s.Delete("stop_time")
			return errors.New("fail to PortalLogin")
		}

	}

	return nil

}

// 用户智慧用电相关
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
			Content:    OPQBot.SendTypeTextMsgContent{Content: "未知错误(请及时反馈或重试)"},
		})
		return

	} else {
		err = zhydUser.Login()
		if err != nil {
			log.Println("获取验证码状态失败: ", err)
			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "未知错误(请及时反馈或重试)"},
			})
			return
		}

		isLogged, err := zhydUser.IsLogged()
		if isLogged {

			if packet.Content == "/宿舍用电" {
				des, err := zhydUser.GetDormElectricity()
				if err != nil {
					log.Println("获取余电额度失败: ", err)
					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
					})
					return
				}

				for _, de := range des {
					deStr := fmt.Sprintf("宿舍用电(%s): \n\t\t宿舍楼: %s \n\t\t房间: %s \n\t\t余电: %v 度 \n\t\t余额: %v 元", de.Name, de.BuildName, de.Room, de.Electricity, de.Balance)

					// todo

					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: deStr},
					})

				}

				return
			} else if packet.Content == "/历史用电" || packet.Content == "/历史用电折线图" {

				eds, err := zhydUser.GetElectricityDetails()
				if err != nil {
					log.Println("获取历史用电失败: ", err)
					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
					})
				}

				if packet.Content == "/历史用电" {
					for _, ed := range eds {
						edStr := fmt.Sprintf("历史用电(%s):\n", ed.Name)

						for _, v := range ed.Details {
							edStr += "\t" + v.Time.Format("2006-01-02") + ": " + fmt.Sprint(v.Value) + " 度\n"
						}

						OPQ.Send(OPQBot.SendMsgPack{
							SendToType: OPQBot.SendToTypeFriend,
							ToUserUid:  packet.FromUin,
							Content:    OPQBot.SendTypeTextMsgContent{Content: edStr},
						})
					}
				} else {

					for _, ed := range eds {
						var edTimes []time.Time
						var edValues []float64
						for _, v := range ed.Details {
							edTimes = append(edTimes, v.Time)
							edValues = append(edValues, v.Value)
						}

						graph := chart.Chart{
							Title:      "Daily electricity consumption statistics chart" + "(Nearly " + fmt.Sprint(len(edTimes)) + " days)",
							TitleStyle: chart.Style{FontSize: 8},
							XAxis: chart.XAxis{
								Name:           "Date",
								ValueFormatter: chart.TimeHourValueFormatter,
							},
							YAxis: chart.YAxis{
								Name: "kW·h",
							},
							Series: []chart.Series{
								chart.TimeSeries{
									XValues: edTimes,
									YValues: edValues,
								},
							},
						}

						b := bytes.NewBuffer([]byte{})
						graph.Render(chart.PNG, b) //where graph is my chart.Chart{...}
						byteArray := b.Bytes()

						OPQ.Send(OPQBot.SendMsgPack{
							SendToType: OPQBot.SendToTypeFriend,
							ToUserUid:  packet.FromUin,
							Content:    OPQBot.SendTypePicMsgByBase64Content{Content: "近" + fmt.Sprint(len(edTimes)) + "天每日用电量统计图(" + ed.Name + ")", Base64: base64.StdEncoding.EncodeToString(byteArray)},
						})
					}
				}

				return

			} else if packet.Content == "/充电记录" {

				crs, err := zhydUser.GetChargeRecords()
				if err != nil {
					log.Println("获取充值记录失败: ", err)
					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
					})
					return
				}

				for _, cr := range crs {
					crStr := fmt.Sprintf("充电记录(%s):\n", cr.Device)

					for _, v := range cr.Mx {
						crStr += "\t" + v.Accounttime.Time.Format("2006-01-02") + ": " + v.Inmoney + " 元\n"
					}

					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: crStr},
					})

				}

				return

			} else {

				log.Println("似乎未登陆: ", err)
				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "未知错误(请及时反馈或重试)"},
				})

				return
			}
		}
	}
}

// 用户一卡通相关
func UserOneCard(user model.User, s session.Session, packet *OPQBot.FriendMsgPack) {

	status, _ := s.GetString("status")

	if packet.Content == "/一卡通" {

		OPQ.Send(OPQBot.SendMsgPack{
			SendToType: OPQBot.SendToTypeFriend,
			ToUserUid:  packet.FromUin,
			Content:    OPQBot.SendTypeTextMsgContent{Content: "正在查询...请稍后!"},
		})

		rte, err := MySecUser.GetOneCardBalance(user.StuID)

		if err != nil {
			log.Println("查询一卡通余额失败: ", err)

			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
			})

			return
		}

		rteContent := fmt.Sprintf("一卡通: \n\t 余额: %v 元 \n\t 本月消费: %v 元 \n\t 上月消费: %v 元", rte.Obj.Balance, rte.Obj.ThisMonthMoney, rte.Obj.LastMonthMoney)

		OPQ.Send(OPQBot.SendMsgPack{
			SendToType: OPQBot.SendToTypeFriend,
			ToUserUid:  packet.FromUin,
			Content:    OPQBot.SendTypeTextMsgContent{Content: rteContent},
		})

	} else if packet.Content == "/充卡记录" || status == "GetOneCardChargeRecords" {

		maxPageSize := 30

		if status != "GetOneCardChargeRecords" {

			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "正在查询...请稍后!"},
			})

			rte, err := MySecUser.GetOneCardChargeRecords(user.StuID, 1, uint(maxPageSize))
			if err != nil {
				log.Println("查询一卡通充值记录失败: ", err)

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
				})

				return
			}

			if rte.Count > int64(maxPageSize) {
				lastRte, err := MySecUser.GetOneCardChargeRecords(user.StuID, 1, uint(rte.Count))
				if err != nil {
					log.Println("查询一卡通消费记录失败: ", err)

					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
					})

					return
				}

				s.Set("status", "GetOneCardChargeRecords")
				s.Set("OneCardChargeRecords", lastRte)

				rteContent := "一卡通充值记录: \n"

				for _, v := range lastRte.Obj[0:maxPageSize] {
					rteContent = rteContent + "\t" + v.GeneraCardRechargeRecordTransactionTime + ": " + v.GeneraCardRechargeRecordTransactionAdress + " " + v.GeneraCardRechargeRecordTransactionType + " " + v.GeneraCardRechargeRecordTransactionBalance + " 元\n"
				}

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: rteContent},
				})

				s.Set("pageindex", maxPageSize)

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "如需要查看更多记录, 请发送 /下一页 或发送其他任意消息停止本次操作!"},
				})

			} else {
				rteContent := "一卡通充值记录: \n"

				for _, v := range rte.Obj {
					rteContent = rteContent + "\t" + v.GeneraCardRechargeRecordTransactionTime + ": " + v.GeneraCardRechargeRecordTransactionAdress + " " + v.GeneraCardRechargeRecordTransactionType + " " + v.GeneraCardRechargeRecordTransactionBalance + " 元\n"
				}

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: rteContent},
				})
			}

		} else {

			if packet.Content != "/下一页" {
				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "已结束本次操作~"},
				})

				s.Delete("status")
				s.Delete("OneCardChargeRecords")
				s.Delete("pageindex")

			}

			lastRteIf, _ := s.Get("OneCardChargeRecords")

			lastRte, ok := lastRteIf.(sec.GetOneCardChargeRecordsRte)
			if !ok {
				return
			}

			index, _ := s.GetInt("pageindex")

			rteContent := "一卡通消费记录: \n"

			if index+maxPageSize > len(lastRte.Obj) {

				for _, v := range lastRte.Obj[index : len(lastRte.Obj)-1] {
					rteContent = rteContent + "\t" + v.GeneraCardRechargeRecordTransactionTime + ": " + v.GeneraCardRechargeRecordTransactionAdress + " " + v.GeneraCardRechargeRecordTransactionType + " " + v.GeneraCardRechargeRecordTransactionBalance + " 元\n"
				}

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: rteContent},
				})

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "已无更多结果了哦~"},
				})

				s.Delete("status")
				s.Delete("OneCardChargeRecords")
				s.Delete("pageindex")

				return
			}

			for _, v := range lastRte.Obj[index : index+maxPageSize] {
				rteContent = rteContent + "\t" + v.GeneraCardRechargeRecordTransactionTime + ": " + v.GeneraCardRechargeRecordTransactionAdress + " " + v.GeneraCardRechargeRecordTransactionType + " " + v.GeneraCardRechargeRecordTransactionBalance + " 元\n"

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: rteContent},
				})

				s.Set("pageindex", maxPageSize+index)

			}

			return
		}

	} else if packet.Content == "/一卡通消费记录" || status == "GetOneCardConsumeRecords" {

		maxPageSize := 30

		if status != "GetOneCardConsumeRecords" {

			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "正在查询...请稍后!"},
			})

			rte, err := MySecUser.GetOneCardConsumeRecords(user.StuID, 1, uint(maxPageSize))
			if err != nil {
				log.Println("查询一卡通消费记录失败: ", err)

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
				})

				return
			}

			if rte.Count > int64(maxPageSize) {
				lastRte, err := MySecUser.GetOneCardConsumeRecords(user.StuID, 1, uint(rte.Count))
				if err != nil {
					log.Println("查询一卡通消费记录失败: ", err)

					OPQ.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeFriend,
						ToUserUid:  packet.FromUin,
						Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
					})

					return
				}

				s.Set("status", "GetOneCardConsumeRecords")
				s.Set("OneCardConsumeRecords", lastRte)

				rteContent := "一卡通消费记录: \n"

				for _, v := range lastRte.Obj[0:maxPageSize] {
					rteContent = rteContent + "\t" + v.GeneraCardConsumeRecordTransactionTime + ": " + v.GeneraCardConsumeRecordTransactionAdress + " " + v.GeneraCardConsumeRecordTransactionType + " " + v.GeneraCardConsumeRecordTransactionMoney + " 元\n"
				}

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: rteContent},
				})

				s.Set("pageindex", maxPageSize)

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "如需要查看更多记录, 请发送 /下一页 或发送其他任意消息停止本次操作!"},
				})

			} else {
				rteContent := "一卡通消费记录: \n"

				for _, v := range rte.Obj {
					rteContent = rteContent + "\t" + v.GeneraCardConsumeRecordTransactionTime + ": " + v.GeneraCardConsumeRecordTransactionAdress + " " + v.GeneraCardConsumeRecordTransactionType + " " + v.GeneraCardConsumeRecordTransactionMoney + " 元\n"
				}

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: rteContent},
				})
			}

		} else {

			if packet.Content != "/下一页" {
				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "已结束本次操作~"},
				})

				s.Delete("status")
				s.Delete("OneCardConsumeRecords")
				s.Delete("pageindex")

			}

			lastRteIf, _ := s.Get("OneCardConsumeRecords")

			lastRte, ok := lastRteIf.(sec.GetOneCardConsumeRecordsRte)
			if !ok {
				return
			}

			index, _ := s.GetInt("pageindex")

			rteContent := "一卡通消费记录: \n"

			if index+maxPageSize > len(lastRte.Obj) {

				for _, v := range lastRte.Obj[index : len(lastRte.Obj)-1] {
					rteContent = rteContent + "\t" + v.GeneraCardConsumeRecordTransactionTime + ": " + v.GeneraCardConsumeRecordTransactionAdress + " " + v.GeneraCardConsumeRecordTransactionType + " " + v.GeneraCardConsumeRecordTransactionMoney + " 元\n"
				}

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: rteContent},
				})

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: "已无更多结果了哦~"},
				})

				s.Delete("status")
				s.Delete("OneCardConsumeRecords")
				s.Delete("pageindex")

				return
			}

			for _, v := range lastRte.Obj[index : index+maxPageSize] {
				rteContent = rteContent + "\t" + v.GeneraCardConsumeRecordTransactionTime + ": " + v.GeneraCardConsumeRecordTransactionAdress + " " + v.GeneraCardConsumeRecordTransactionType + " " + v.GeneraCardConsumeRecordTransactionMoney + " 元\n"
			}

			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: rteContent},
			})

			s.Set("pageindex", maxPageSize+index)

		}

		return

	}
}

func UserExam(user model.User, s session.Session, packet *OPQBot.FriendMsgPack) {
	if packet.Content == "/考试安排" {
		rte, err := MySecUser.GetExamArrangements(user.StuID, SchoolYear, SchoolTerm)

		if err != nil {
			log.Println("查询一卡通余额失败: ", err)

			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "错误(请及时反馈): " + err.Error()},
			})

			return
		}

		rteContent := "本学期考试安排(仅供参考): \n\n"

		timeLayout := "15:04:05"

		//定义一个正12小时
		ad, _ := time.ParseDuration("12h")

		for _, v := range rte.Obj {

			var sTimeStr string
			var eTimeStr string

			// 有些时间录错了...手动修复阴间时间
			sTime, e := time.Parse(timeLayout, v.ExaminationStartTime)
			if e != nil {
				sTimeStr = v.ExaminationStartTime
			} else {
				if sTime.Hour() < 8 && sTime.Hour() > 0 {
					sTimeStr = sTime.Add(ad).Format(timeLayout)
				} else {
					sTimeStr = v.ExaminationStartTime
				}
			}

			// 有些时间录错了...手动修复阴间时间
			eTime, e := time.Parse(timeLayout, v.ExaminationEndTime)
			if e != nil {
				eTimeStr = v.ExaminationEndTime
			} else {
				if eTime.Hour() < 8 && eTime.Hour() > 0 {
					eTimeStr = eTime.Add(ad).Format(timeLayout)
				} else {
					eTimeStr = v.ExaminationEndTime
				}
			}

			rteContent = rteContent + "\t\t科目: " + v.ExaminationCourseName + "\n\t\t考试日期: " + v.ExaminationTime + "\n\t\t开始时间: " + sTimeStr + "\n\t\t结束时间: " + eTimeStr + "\n\t\t地点: " + v.ExaminationAdressName + "\n\n"
		}

		OPQ.Send(OPQBot.SendMsgPack{
			SendToType: OPQBot.SendToTypeFriend,
			ToUserUid:  packet.FromUin,
			Content:    OPQBot.SendTypeTextMsgContent{Content: rteContent},
		})

	}

}

// 处理群消息
func handleGroupMsg(botQQ int64, packet *OPQBot.GroupMsgPack) {
	packet.Next(botQQ, packet)
}

// 处理私聊消息
func handleFriendMsg(botQQ int64, packet *OPQBot.FriendMsgPack) {

	s := OPQ.Session.SessionStart(packet.FromUin)

	status, _ := s.GetString("status")
	stopTime, _ := s.GetInt("stop_time")

	// 排除机器人自己
	if packet.FromUin == botQQ {
		return
	}

	isInGroup := false
	// 验证是否为群内人员
	for _, v := range MemberUinList {
		if v == packet.FromUin {
			isInGroup = true
			break
		}
	}

	if !isInGroup {
		OPQ.Send(OPQBot.SendMsgPack{
			SendToType: OPQBot.SendToTypeFriend,
			ToUserUid:  packet.FromUin,
			Content:    OPQBot.SendTypeTextMsgContent{Content: "您无权使用洛洛, 请加入群(oh-my-lit)后重试!"},
		})
		s.Delete("status")
		return
	}

	// 查找用户
	var user model.User
	err := model.DB.Where("qq = ?", packet.FromUin).First(&user).Error
	if err != nil {
		log.Println(err)
	}

	// 无条件暂停
	if (packet.Content == "/停止" && status != "") || stopTime >= 2 {
		s.Delete("status")
		s.Delete("step")
		s.Delete("username")
		s.Delete("password")
		s.Delete("sec_user")
		s.Delete("stop_time")
		s.Delete("OneCardConsumeRecords")
		s.Delete("OneCardChargeRecords")
		s.Delete("pageindex")
		OPQ.Send(OPQBot.SendMsgPack{
			SendToType: OPQBot.SendToTypeFriend,
			ToUserUid:  packet.FromUin,
			Content:    OPQBot.SendTypeTextMsgContent{Content: "注意: 本次操作已意外中断!"},
		})

		return
	}

	if strings.Contains(packet.Content, "菜单") || strings.Contains(packet.Content, "帮助") || strings.Contains(packet.Content, "功能") {

		content := "基础功能: \n\t/绑定 - 绑定智慧门户帐号 \n\t/状态 - 查看绑定状态 \n\t/解绑 - 彻底解除绑定 \n\n智慧控电: \n\t/宿舍用电 - 查询当前宿舍剩余用电 \n\t/历史用电 - 查询宿舍历史用电情况 \n\t/历史用电折线图 - 生成每日用电量折线图 \n\t/充电记录 - 查询宿舍电量充值记录 \n\n一卡通: \n\t/一卡通 - 查询卡内余额 \n\t/一卡通消费记录 - 查询一卡通历史消费记录 \n\t/充卡记录 - 查询一卡通充值记录 \n\n考试相关: \n\t/考试安排 - 查询本学期考试安排 "

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
	} else if status == "GetOneCardConsumeRecords" || status == "GetOneCardChargeRecords" {
		UserOneCard(user, s, packet)
	} else if status == "" {

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
					Content:    OPQBot.SendTypeTextMsgContent{Content: "您已绑定学号: " + user.StuID + " ! \n\n如需解除绑定, 请发送 /解绑 !"},
				})
			}
			return
		}

		// 未绑定的情况
		if user.QQ == 0 {

			OPQ.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeFriend,
				ToUserUid:  packet.FromUin,
				Content:    OPQBot.SendTypeTextMsgContent{Content: "您未绑定任何学号! \n\n如需进行绑定操作, 请发送 /绑定 !"},
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

				content := fmt.Sprintf("状态: \n\t姓名: %s \n\t学号: %s \n\t智慧门户: 已绑定", user.Name, user.StuID)

				OPQ.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeFriend,
					ToUserUid:  packet.FromUin,
					Content:    OPQBot.SendTypeTextMsgContent{Content: content},
				})

			} else if packet.Content == "/宿舍用电" || packet.Content == "/历史用电" || packet.Content == "/充电记录" || packet.Content == "/历史用电折线图" {
				UserZhyd(user, s, packet)
			} else if packet.Content == "/一卡通" || packet.Content == "/一卡通消费记录" || packet.Content == "/充卡记录" {
				UserOneCard(user, s, packet)
			} else if packet.Content == "/考试安排" {
				UserExam(user, s, packet)
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
	OPQ.SetMaxRetryCount(500)

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

		var tmpUin int64

		MemberUinList = []int64{}

		for {
			gi, err := OPQ.GetGroupMemberList(config.ProConf.QQGroup, tmpUin)
			if err != nil {
				log.Println(err)
			} else {
				for _, v := range gi.MemberList {
					MemberUinList = append(MemberUinList, v.MemberUin)
				}

			}

			maxRteSize := 250

			if len(gi.MemberList) == maxRteSize {
				tmpUin = gi.MemberList[maxRteSize-1].MemberUin
			} else {
				break
			}

		}

		// if len(GroupInfo.MemberList) != 0 {
		// 	break
		// }

		log.Println("更新群 (", config.ProConf.QQGroup, ") 资料成功! 当前人数", len(MemberUinList))

	})
	if err != nil {
		log.Println(err.Error())
	}
	err = OPQ.AddEvent(OPQBot.EventNameOnDisconnected, func() {
		log.Println(err)
		log.Println("连接断开！！")
	})
	if err != nil {
		log.Println(err.Error())
	}
	err = OPQ.AddEvent(OPQBot.EventNameOnOther, func(botQQ int64, e interface{}) {
		log.Println(e)
	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupSystemNotify, func(botQQ int64, e *OPQBot.GroupSystemNotifyPack) {
		log.Println(e)
	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupRevoke, func(botQQ int64, e *OPQBot.GroupRevokePack) {
		log.Println(e)
	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupJoin, func(botQQ int64, e *OPQBot.GroupJoinPack) {

		log.Println(e)

		log.Println(e.EventMsg)
		log.Println(e.EventData)

		log.Println("加群事件:", e.EventMsg.FromUin, e.EventData.UserID)

		if e.EventMsg.FromUin == config.ProConf.QQGroup {
			MemberUinList = append(MemberUinList, e.EventData.UserID)
			log.Println("已授予权限!")
		}

	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupAdmin, func(botQQ int64, e *OPQBot.GroupAdminPack) {
		log.Println(e)
		log.Println(e.EventMsg)
		log.Println(e.EventData)
	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupExit, func(botQQ int64, e *OPQBot.GroupExitPack) {
		log.Println(e)

		log.Println(e.EventMsg)
		log.Println(e.EventData)

		log.Println("退群事件:", e.EventMsg.FromUin, e.EventData.UserID)

		if e.EventMsg.FromUin == config.ProConf.QQGroup {

			for i, v := range MemberUinList {
				if v == e.EventData.UserID {
					MemberUinList = append(MemberUinList[:i], MemberUinList[i+1:]...)
					break
				}
			}

			log.Println("已移除权限!")
		}

		log.Println(e.EventMsg)
		log.Println(e.EventData)
	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupExitSuccess, func(botQQ int64, e *OPQBot.GroupExitSuccessPack) {
		log.Println(e)
		log.Println(e.EventMsg)
		log.Println(e.EventData)
	})
	err = OPQ.AddEvent(OPQBot.EventNameOnGroupAdminSysNotify, func(botQQ int64, e *OPQBot.GroupAdminSysNotifyPack) {
		log.Println(e)
	})
	if err != nil {
		log.Println(err.Error())
	}

	OPQ.Wait()
}
