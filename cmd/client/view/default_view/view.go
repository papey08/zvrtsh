package defaultview

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"zvrtsh/cmd/client/app"
	"zvrtsh/internal/model"
	"zvrtsh/internal/ports/httpserver"
)

const (
	wrongInput = "Кажется вы ввели чепуху. Попробуйте снова:"
)

type ViewImpl struct {
	ctx context.Context
	app app.App
}

func (v *ViewImpl) Start() {
	v.SetHost()

	fmt.Println("Введите 1, если у вас уже есть аккаунт в zvrtsh и вы хотите залогиниться, либо введите 2, если хотите зарегистрировать новый аккаунт:")
	if isInputOne() {
		v.Login()
	} else {
		v.Registration()
	}
}

func (v *ViewImpl) SetHost() {
	fmt.Println("Привет, добро пожаловать в zvrtsh! Пожалуйста введите адрес сервера:")
	var host string
	for {
		fmt.Scan(&host)
		_, err := url.ParseRequestURI(host)
		if err != nil {
			fmt.Println(wrongInput)
			continue
		}
		break
	}
	v.app = app.New(host)
	fmt.Println("Большое спасибо!")
}

func (v *ViewImpl) Registration() {
	for {
		var name, password string
		fmt.Println("Придумайте и введите логин:")
		fmt.Scan(&name)
		fmt.Println("Придумайте и введите пароль:")
		fmt.Scan(&password)
		if err := v.app.RegisterUser(v.ctx, name, password); err != nil {
			fmt.Println("Зарегистрироваться не получилось(")
			fmt.Printf("Текст ошибки: %s\n", err.Error())
		} else {
			fmt.Println("Поздравляем! Регистрация прошла успешно. Пожалуйста, запомните логин и пароль, механизмов восстановления аккаунта, разумеется, нет!")
			break
		}
	}

	fmt.Println("Хотите присоединиться к существующей игре или создать свою?")
	fmt.Println("Введите 1, чтобы получить свой invite-код, либо введите 2, чтобы создать игру:")
	if isInputOne() {
		v.ShowInviteCode()
	} else {
		v.CreateSession()
	}
}

func (v *ViewImpl) Login() {
	for {
		var name, password string
		fmt.Println("Введите логин:")
		fmt.Scan(&name)
		fmt.Println("Введите пароль:")
		fmt.Scan(&password)
		if err := v.app.LoginUser(v.ctx, name, password); err != nil {
			fmt.Println("Войти в аккаунт не получилось(")
			fmt.Printf("Текст ошибки: %s\n", err.Error())
		} else {
			fmt.Println("Поздравляем! Вы успешно вошли в свой аккаунт")
			break
		}
	}

	fmt.Println("Хотите присоединиться к существующей игре или создать свою?")
	fmt.Println("Введите 1, чтобы получить свой invite-код, либо введите 2, чтобы создать игру:")
	if isInputOne() {
		v.ShowInviteCode()
	} else {
		v.CreateSession()
	}
}

func (v *ViewImpl) ShowInviteCode() {
	code, err := v.app.GetUserInviteCode(v.ctx)
	if err != nil {
		fmt.Println("Странно, не получилось найти ваш invite-код, попробуйте залогиниться снова.")
		fmt.Printf("Текст ошибки: %s\n", err.Error())
		return
	} else {
		fmt.Println("Ваш invite-код:")
		fmt.Printf("\n%s\n\n", code)
		fmt.Println("Покажите его другому игроку, чтобы он пригласил вас в игру")
		v.EnterSession()
	}
}

func (v *ViewImpl) EnterSession() {
	fmt.Println("Попросите у игрока, который вас пригласил, id игры, и введите его:")
	var code string
	for {
		fmt.Scan(&code)
		if stepsAmount, err := v.app.GetSession(v.ctx, code); err != nil {
			fmt.Println("Странно, что-то пошло не так, попробуйте ввести другой код.")
			fmt.Printf("Текст ошибки: %s\n", err.Error())
		} else {
			v.app.SessionCode = code
			v.app.StepsAmount = stepsAmount
			v.app.CurrentStep = 0
			break
		}

	}
	v.GetQuestion()
}

func (v *ViewImpl) CreateSession() {
	for {
		fmt.Println("Введите invite-код игрока, с которым хотите сыграть:")
		var code string
		fmt.Scan(&code)
		var stepsAmount int
		for {
			fmt.Println("Введите количество вопросов в игре одним числом:")
			var stepsAmountStr string
			fmt.Scan(&stepsAmountStr)
			var err error
			if stepsAmount, err = strconv.Atoi(stepsAmountStr); err != nil || stepsAmount <= 0 {
				fmt.Println(wrongInput)
			} else {
				break
			}
		}

		sessionCode, err := v.app.CreateSession(v.ctx, code, stepsAmount)
		if err != nil {
			fmt.Println("Странно, что-то пошло не так, попробуйте снова.")
			fmt.Printf("Текст ошибки: %s\n", err.Error())
		} else {
			fmt.Println("Игра успешно создана. Код вашей игры:")
			fmt.Printf("\n%s\n\n", sessionCode)
			fmt.Println("Покажите код другому игроку, чтобы он смог присоединиться к игре.")
			v.app.SessionCode = sessionCode
			v.app.StepsAmount = stepsAmount
			v.app.CurrentStep = 0
			break
		}
	}
	v.GetQuestion()
}

func (v *ViewImpl) GetQuestion() {
	for {
		fmt.Println("Введите ваш вопрос целиком:")
		hiddenQuestion, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		hiddenQuestion = hiddenQuestion[:len(hiddenQuestion)-1]
		fmt.Println("Введите часть вопроса, которая будет показана другому игроку:")
		openQuestion, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		openQuestion = openQuestion[:len(openQuestion)-1]
		if err := v.app.MakeQuestion(v.ctx, v.app.CurrentStep, hiddenQuestion, openQuestion); err != nil {
			fmt.Println("Странно, что-то пошло не так, попробуйте снова.")
			fmt.Printf("Текст ошибки: %s\n", err.Error())
		} else {
			fmt.Println("Спасибо, ваш вопрос записан.")
			break
		}
	}
	v.ShowQuestion()
}

func (v *ViewImpl) ShowQuestion() {
	var openQuestion string
	var firstLog = true
	for {
		var err error
		openQuestion, err = v.app.GetQuestion(v.ctx, v.app.CurrentStep)
		if err != nil && strings.Contains(err.Error(), model.ErrStepForbidden.Error()) {
			if firstLog {
				fmt.Println("Ждём, пока второй игрок составит свой вопрос…")
				firstLog = false
			}
			time.Sleep(time.Second * 5)
			continue
		} else if err != nil {
			fmt.Println("Странно, что-то пошло не так. Извините пожалуйста.")
			fmt.Printf("Текст ошибки: %s\n", err.Error())
			return
		} else {
			break
		}
	}

	fmt.Printf("Вопрос: %s\n", openQuestion)
	v.GetAnswer()
}

func (v *ViewImpl) GetAnswer() {
	for {
		fmt.Println("Пожалуйста, введите ответ на вопрос:")
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answer = answer[:len(answer)-1]
		if err := v.app.MakeAnswer(v.ctx, v.app.CurrentStep, answer); err != nil {
			fmt.Println("Странно, что-то пошло не так, попробуйте снова.")
			fmt.Printf("Текст ошибки: %s\n", err.Error())
		} else {
			fmt.Println("Спасибо, ваш ответ записан")
			break
		}
	}

	v.app.CurrentStep++
	if v.app.CurrentStep == v.app.StepsAmount {
		v.ShowResults()
	} else {
		v.GetQuestion()
	}
}

func (v *ViewImpl) ShowResults() {
	fmt.Println("Похоже, вы закончили игру.")
	var data httpserver.GetSessionData
	var firstLog = true
	for {
		var err error
		data, err = v.app.GetResults(v.ctx)
		if err != nil && strings.Contains(err.Error(), model.ErrSessionUndone.Error()) {
			if firstLog {
				fmt.Println("Ждём, пока второй игрок ответит…")
				firstLog = false
			}
			time.Sleep(time.Second * 5)
		}
		if err != nil {
			fmt.Println("Странно, что-то пошло не так. Извините пожалуйста.")
			fmt.Printf("Текст ошибки: %s\n", err.Error())
			return
		}
		break
	}

	fmt.Println("\nЛист 1\n")
	time.Sleep(time.Second * 3)
	for i, step := range data.Sheets[0].Steps {
		fmt.Printf("Вопрос %d: %s\n", i+1, step.HiddenQuestion)
		time.Sleep(time.Second * 3)
		fmt.Printf("Ответ: %s\n\n", step.Answer)
		time.Sleep(time.Second * 3)
	}

	fmt.Println("Лист 2\n")
	time.Sleep(time.Second * 3)
	for i, step := range data.Sheets[1].Steps {
		fmt.Printf("Вопрос %d: %s\n", i+1, step.HiddenQuestion)
		time.Sleep(time.Second * 3)
		fmt.Printf("Ответ: %s\n\n", step.Answer)
		time.Sleep(time.Second * 3)
	}

	fmt.Println("Спасибо, что сыграли в zvrtsh! Надеюсь вы не сильно страдали😄😄😄😄")
	return
}

func isInputOne() bool {
	for {
		var ans string
		fmt.Scan(&ans)
		if intAns, _ := strconv.Atoi(ans); intAns == 1 {
			return true
		} else if intAns == 2 {
			return false
		} else {
			fmt.Println(wrongInput)
			continue
		}
	}
}
