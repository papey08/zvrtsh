import { useState, type FormEvent } from "react";
import { api, call, humanize } from "../api/client";
import { Logo } from "../components/ui";
import { useAuth } from "../lib/auth";
import { validateName, validatePassword } from "../lib/game";

type Mode = "login" | "register";

export function AuthScreen({ invited }: { invited: boolean }) {
  const { login } = useAuth();
  const [mode, setMode] = useState<Mode>(invited ? "register" : "login");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (mode === "register") {
      const invalid = validateName(name) ?? validatePassword(password);
      if (invalid) {
        setError(invalid);
        return;
      }
    }

    setBusy(true);
    try {
      if (mode === "register") {
        await call(api.POST("/users", { body: { name, password } }));
      }
      const { token } = await call(api.POST("/auth/token", { body: { name, password } }));
      login({ name, token });
    } catch (err) {
      setError(humanize(err));
      setBusy(false);
    }
  }

  function switchMode(next: Mode) {
    setMode(next);
    setError(null);
  }

  return (
    <div className="auth">
      <section className="auth__hero">
        <Logo big />
        <h1 className="auth__title">
          Пишите вопросы.
          <br />
          Загибайте лист.
          <br />
          <span className="auth__title-accent">Отвечайте вслепую.</span>
        </h1>
        <ol className="rules">
          <li>
            <b>Напишите вопрос</b> и загните лист — следующий игрок увидит только его начало, например «Что?»
          </li>
          <li>
            <b>Ответьте наугад</b> на начало чужого вопроса из перемешанных листов и загните ответ
          </li>
          <li>
            <b>Разверните листы</b>, когда кончатся шаги, и прочитайте, что получилось
          </li>
        </ol>
        <DemoSheet />
      </section>

      <section className="auth__panel">
        <form className="card auth__card" onSubmit={submit} noValidate>
          {invited && <p className="auth__invited">Вас пригласили в партию — войдите, чтобы присоединиться</p>}
          <div className="tabs" role="tablist">
            <button
              type="button"
              role="tab"
              aria-selected={mode === "login"}
              className={mode === "login" ? "tabs__tab is-active" : "tabs__tab"}
              onClick={() => switchMode("login")}
            >
              Вход
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={mode === "register"}
              className={mode === "register" ? "tabs__tab is-active" : "tabs__tab"}
              onClick={() => switchMode("register")}
            >
              Регистрация
            </button>
            <span className={`tabs__pill tabs__pill--${mode}`} aria-hidden="true" />
          </div>

          <label className="field">
            <span className="field__label">Имя</span>
            <input
              className="input"
              value={name}
              onChange={(e) => setName(e.target.value.trim())}
              autoComplete="username"
              placeholder="например, pushkin_1799"
              maxLength={30}
              autoFocus
              required
            />
            {mode === "register" && <span className="field__hint">4–30 символов: латиница, цифры и _</span>}
          </label>

          <label className="field">
            <span className="field__label">Пароль</span>
            <input
              className="input"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete={mode === "login" ? "current-password" : "new-password"}
              placeholder="••••••"
              maxLength={30}
              required
            />
          </label>

          {error && <p className="form-error">{error}</p>}

          <button className="btn btn--primary btn--wide" disabled={busy || !name || !password}>
            {busy ? "Секунду…" : mode === "login" ? "Войти" : "Создать аккаунт"}
          </button>
        </form>
      </section>
    </div>
  );
}

/** A looping illustration of the game on the auth screen. */
function DemoSheet() {
  return (
    <div className="demo" aria-hidden="true">
      <div className="paper demo__back" />
      <div className="paper demo__sheet">
        <div className="strips">
          <span className="strip" style={{ ["--i" as string]: 0 }} />
          <span className="strip" style={{ ["--i" as string]: 1 }} />
          <span className="strip" style={{ ["--i" as string]: 2 }} />
        </div>
        <div className="demo__open">Что?</div>
        <div className="demo__answer">
          <span className="demo__write">Пуговицы</span>
        </div>
      </div>
      <div className="demo__note">
        <span className="demo__q">Что я ел на завтрак?</span>
        <span className="demo__a">— Пуговицы</span>
      </div>
    </div>
  );
}
