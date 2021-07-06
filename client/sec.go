package client

import (
	"bytes"
	"fmt"
	"io"
	"lit-life-bot/config"
	"log"
	"os"
	"time"

	"github.com/icepie/oh-my-lit/client/sec"
)

var (
	// 智慧门户主客户端
	MySecUser sec.SecUser
)

func SecStart() {
	log.Println("智慧门户测试")

	var err error

	MySecUser, err = sec.NewSecUser(config.ProConf.Sec.Username, config.ProConf.Sec.Password)
	if err != nil {
		log.Println("实例化用户失败: ", err)
		return
	}

	isNeedcap, err := MySecUser.IsNeedCaptcha()
	if err != nil {
		log.Println("获取状态信息失败: ", err)
		return
	}

	if isNeedcap {
		log.Println("需要验证码")

		pix, err := MySecUser.GetCaptche()

		if err != nil {
			log.Println("获取验证码失败: ", err)
		}

		out, err := os.Create("./captcha.jpeg")
		if err != nil {
			log.Fatal(err)
		}
		defer out.Close()

		_, err = io.Copy(out, bytes.NewReader(pix))
		if err != nil {
			log.Fatal(err)
		}

		var capp string
		fmt.Print("验证码(./captcha.jpeg): ")
		fmt.Scanf("%s", &capp)

		err = MySecUser.LoginWithCap(capp)
		if err != nil {
			log.Fatal("登陆失败: ", err)
		}

	} else {
		log.Println("不需要验证码")
		err = MySecUser.Login()
		if err != nil {
			log.Fatal("登陆失败: ", err)
		}
	}

	// 门户登陆
	MySecUser.PortalLogin()

	// 保活
	for {
		if MySecUser.IsLogged() {
			if MySecUser.IsPortalLogged() {
				log.Println("[sec]: work fine!")
			} else {
				log.Println("[sec]: portal refreshing!")
				MySecUser.PortalLogin()
			}
		} else {
			err = MySecUser.Login()
			if err != nil {
				log.Println("登陆失败: ", err)
			}
		}

		time.Sleep(time.Second * 10)
	}

	// if secUser.IsLogged() {

	// 	// 进行门户登陆
	// 	secUser.PortalLogin()

	// 	if secUser.IsPortalLogged() {

	// 		test, err := secUser.GetCurrentMember()
	// 		if err != nil {
	// 			log.Fatal("获取个人信息失败: ", err)
	// 		}

	// 		log.Println("欢迎!", test.Obj.MemberNickname, test.Obj.RoleList[0].RoleName)
	// 		log.Println("上次登陆时间", test.Obj.LastLoginTime)

	// 		t1, err := secUser.GetStudent(secUser.Username)
	// 		if err != nil {
	// 			log.Fatal("查询学生信息失败: ", err)
	// 		}

	// 		log.Println(t1)

	// 		t2, err := secUser.GetClassmates(secUser.Username)
	// 		if err != nil {
	// 			log.Fatal("查询同班同学列表失败: ", err)
	// 		}

	// 		log.Println(t2)

	// 		t3, err := secUser.GetClassmatesDetail(secUser.Username)
	// 		if err != nil {
	// 			log.Fatal("查询同班同学信息失败: ", err)
	// 		}

	// 		log.Println(t3)

	// 		t4, err := secUser.GetOneCardBalance(secUser.Username)
	// 		if err != nil {
	// 			log.Fatal("查询一卡通余额失败: ", err)
	// 		}

	// 		log.Println(t4)

	// 		t5, err := secUser.GetOneCardChargeRecords(secUser.Username, 1, 10)
	// 		if err != nil {
	// 			log.Fatal("查询一卡通充值记录失败: ", err)
	// 		}

	// 		log.Println(t5)

	// 		t6, err := secUser.GetOneCardConsumeRecords(secUser.Username, 1, 10)
	// 		if err != nil {
	// 			log.Fatal("查询一卡通充值记录失败: ", err)
	// 		}

	// 		log.Println(t6)
	// 	}

	// }
}
