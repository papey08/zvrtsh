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

	fmt.Println()
	printString("Введите 1, если у вас уже есть аккаунт в zvrtsh и вы хотите залогиниться, либо введите 2, если хотите зарегистрировать новый аккаунт:")
	if isInputOne() {
		v.Login()
	} else {
		v.Registration()
	}
}

func (v *ViewImpl) SetHost() {
	printString("Привет, добро пожаловать в zvrtsh! Пожалуйста введите адрес сервера:")
	var host string
	for {
		fmt.Scan(&host)
		_, err := url.ParseRequestURI(host)
		if err != nil {
			printString(wrongInput)
			continue
		}
		break
	}
	v.app = app.New(host)
	printString("Большое спасибо!")
}

func (v *ViewImpl) Registration() {
	fmt.Println()
	for {
		var name, password string
		printString("Придумайте и введите логин:")
		fmt.Scan(&name)
		printString("Придумайте и введите пароль:")
		fmt.Scan(&password)
		if err := v.app.RegisterUser(v.ctx, name, password); err != nil {
			printString("Зарегистрироваться не получилось(")
			printString(fmt.Sprintf("Текст ошибки: %s", err.Error()))
		} else {
			printString("Поздравляем! Регистрация прошла успешно. Пожалуйста, запомните логин и пароль, механизмов восстановления аккаунта, разумеется, нет!")
			break
		}
	}

	printString("Хотите присоединиться к существующей игре или создать свою?")
	printString("Введите 1, чтобы получить свой invite-код, либо введите 2, чтобы создать игру:")
	if isInputOne() {
		v.ShowInviteCode()
	} else {
		v.CreateSession()
	}
}

func (v *ViewImpl) Login() {
	fmt.Println()
	for {
		var name, password string
		printString("Введите логин:")
		fmt.Scan(&name)
		printString("Введите пароль:")
		fmt.Scan(&password)
		if err := v.app.LoginUser(v.ctx, name, password); err != nil {
			printString("Войти в аккаунт не получилось(")
			printString(fmt.Sprintf("Текст ошибки: %s", err.Error()))
		} else {
			printString("Поздравляем! Вы успешно вошли в свой аккаунт")
			break
		}
	}

	printString("Хотите присоединиться к существующей игре или создать свою?")
	printString("Введите 1, чтобы получить свой invite-код, либо введите 2, чтобы создать игру:")
	if isInputOne() {
		v.ShowInviteCode()
	} else {
		v.CreateSession()
	}
}

func (v *ViewImpl) ShowInviteCode() {
	fmt.Println()
	code, err := v.app.GetUserInviteCode(v.ctx)
	if err != nil {
		printString("Странно, не получилось найти ваш invite-код, попробуйте залогиниться снова.")
		printString(fmt.Sprintf("Текст ошибки: %s", err.Error()))
		return
	} else {
		printString("Ваш invite-код:")
		printString(fmt.Sprintf("\n%s\n", code))
		printString("Покажите его другому игроку, чтобы он пригласил вас в игру")
		v.EnterSession()
	}
}

func (v *ViewImpl) EnterSession() {
	fmt.Println()
	printString("Попросите у игрока, который вас пригласил, id игры, и введите его:")
	var code string
	for {
		fmt.Scan(&code)
		if stepsAmount, err := v.app.GetSession(v.ctx, code); err != nil {
			printString("Странно, что-то пошло не так, попробуйте ввести другой код.")
			printString(fmt.Sprintf("Текст ошибки: %s", err.Error()))
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
	fmt.Println()
	for {
		printString("Введите invite-код игрока, с которым хотите сыграть:")
		var code string
		fmt.Scan(&code)
		var stepsAmount int
		for {
			printString("Введите количество вопросов в игре одним числом:")
			var stepsAmountStr string
			fmt.Scan(&stepsAmountStr)
			var err error
			if stepsAmount, err = strconv.Atoi(stepsAmountStr); err != nil || stepsAmount <= 0 {
				printString(wrongInput)
			} else {
				break
			}
		}

		sessionCode, err := v.app.CreateSession(v.ctx, code, stepsAmount)
		if err != nil {
			printString("Странно, что-то пошло не так, попробуйте снова.")
			printString(fmt.Sprintf("Текст ошибки: %s", err.Error()))
		} else {
			printString("Игра успешно создана. Код вашей игры:")
			printString(fmt.Sprintf("\n%s\n", sessionCode))
			printString("Покажите код другому игроку, чтобы он смог присоединиться к игре.")
			v.app.SessionCode = sessionCode
			v.app.StepsAmount = stepsAmount
			v.app.CurrentStep = 0
			break
		}
	}
	v.GetQuestion()
}

func (v *ViewImpl) GetQuestion() {
	fmt.Println()
	for {
		printString("Введите ваш вопрос целиком:")
		hiddenQuestion, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		hiddenQuestion = hiddenQuestion[:len(hiddenQuestion)-1]
		printString("Введите часть вопроса, которая будет показана другому игроку:")
		openQuestion, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		openQuestion = openQuestion[:len(openQuestion)-1]
		if err := v.app.MakeQuestion(v.ctx, v.app.CurrentStep, hiddenQuestion, openQuestion); err != nil {
			printString("Странно, что-то пошло не так, попробуйте снова.")
			printString(fmt.Sprintf("Текст ошибки: %s", err.Error()))
		} else {
			printString("Спасибо, ваш вопрос записан.")
			break
		}
	}
	v.ShowQuestion()
}

func (v *ViewImpl) ShowQuestion() {
	fmt.Println()
	var openQuestion string
	var firstLog = true
	for {
		var err error
		openQuestion, err = v.app.GetQuestion(v.ctx, v.app.CurrentStep)
		if err != nil && strings.Contains(err.Error(), model.ErrStepForbidden.Error()) {
			if firstLog {
				printString("Ждём, пока второй игрок составит свой вопрос…")
				firstLog = false
			}
			time.Sleep(time.Second * 5)
			continue
		} else if err != nil {
			printString("Странно, что-то пошло не так. Извините пожалуйста.")
			printString(fmt.Sprintf("Текст ошибки: %s", err.Error()))
			return
		} else {
			break
		}
	}

	printString(fmt.Sprintf("Вопрос: %s", openQuestion))
	v.GetAnswer()
}

func (v *ViewImpl) GetAnswer() {
	fmt.Println()
	for {
		printString("Пожалуйста, введите ответ на вопрос:")
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answer = answer[:len(answer)-1]
		if err := v.app.MakeAnswer(v.ctx, v.app.CurrentStep, answer); err != nil {
			printString("Странно, что-то пошло не так, попробуйте снова.")
			printString(fmt.Sprintf("Текст ошибки: %s", err.Error()))
		} else {
			printString("Спасибо, ваш ответ записан")
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
	fmt.Println()
	printString("Похоже, вы закончили игру.")
	var data httpserver.GetSessionData
	var firstLog = true
	for {
		var err error
		data, err = v.app.GetResults(v.ctx)
		if err != nil && strings.Contains(err.Error(), model.ErrSessionUndone.Error()) {
			if firstLog {
				printString("Ждём, пока второй игрок ответит…")
				firstLog = false
			}
			time.Sleep(time.Second * 5)
		}
		if err != nil {
			printString("Странно, что-то пошло не так. Извините пожалуйста.")
			printString(fmt.Sprintf("Текст ошибки: %s", err.Error()))
			return
		}
		break
	}

	printString("\nЛист 1\n")
	time.Sleep(time.Second * 3)
	for i, step := range data.Sheets[0].Steps {
		printString(fmt.Sprintf("Вопрос %d: %s", i+1, step.HiddenQuestion))
		time.Sleep(time.Second * 3)
		printString(fmt.Sprintf("Ответ: %s\n", step.Answer))
		time.Sleep(time.Second * 3)
	}

	printString("Лист 2\n")
	time.Sleep(time.Second * 3)
	for i, step := range data.Sheets[1].Steps {
		printString(fmt.Sprintf("Вопрос %d: %s", i+1, step.HiddenQuestion))
		time.Sleep(time.Second * 3)
		printString(fmt.Sprintf("Ответ: %s\n", step.Answer))
		time.Sleep(time.Second * 3)
	}

	printString("Спасибо, что сыграли в zvrtsh! Надеюсь вы не сильно страдали😄😄😄😄")
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
			printString(wrongInput)
			continue
		}
	}
}

func printString(s string) {
	for _, c := range s {
		time.Sleep(time.Millisecond * 50)
		fmt.Printf("%c", c)
	}
	fmt.Println()
}
