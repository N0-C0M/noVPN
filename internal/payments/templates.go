package payments

const landingTemplate = `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.BrandName}} · VPN без ожидания</title>
  <style>
    :root {
      --bg:#f4efe5;
      --ink:#1d1d1f;
      --muted:#645f57;
      --card:#fffaf1;
      --panel:#fbf6ed;
      --line:rgba(29,29,31,0.12);
      --accent:#cb4b16;
      --accent-soft:#f48b3c;
      --forest:#1e4d3a;
      --bad:#7f1d1d;
      --shadow:0 24px 70px rgba(75,49,27,0.16);
    }
    * { box-sizing:border-box; }
    body {
      margin:0;
      color:var(--ink);
      background:
        radial-gradient(circle at top left, rgba(244,139,60,0.28), transparent 32%),
        radial-gradient(circle at top right, rgba(30,77,58,0.16), transparent 26%),
        linear-gradient(180deg, #f8f2e8 0%, #efe7da 100%);
      font-family:"Segoe UI Variable","Trebuchet MS","Segoe UI",sans-serif;
    }
    a { color:var(--forest); }
    .shell { max-width:1180px; margin:0 auto; padding:28px 18px 72px; }
    .hero {
      display:grid;
      gap:18px;
      grid-template-columns:1.15fr 0.85fr;
      align-items:start;
    }
    .heroCard, .card {
      background:linear-gradient(180deg, rgba(255,255,255,0.56), rgba(255,255,255,0.86));
      border:1px solid var(--line);
      border-radius:28px;
      box-shadow:var(--shadow);
      backdrop-filter: blur(10px);
    }
    .heroCard { padding:28px; }
    .eyebrow {
      display:inline-flex;
      gap:8px;
      align-items:center;
      padding:8px 12px;
      border-radius:999px;
      background:#fff3df;
      color:var(--accent);
      font-weight:700;
      letter-spacing:0.04em;
      text-transform:uppercase;
      font-size:12px;
    }
    h1 {
      margin:18px 0 12px;
      font-size:clamp(34px, 5vw, 64px);
      line-height:0.96;
      letter-spacing:-0.04em;
      max-width:11ch;
    }
    .lead {
      margin:0;
      color:var(--muted);
      font-size:18px;
      line-height:1.6;
      max-width:44rem;
    }
    .grid {
      margin-top:22px;
      display:grid;
      gap:16px;
      grid-template-columns:repeat(auto-fit,minmax(220px,1fr));
    }
    .card { padding:22px; }
    .price { font-size:34px; font-weight:800; letter-spacing:-0.04em; }
    .muted { color:var(--muted); }
    .planPicker { display:grid; gap:14px; }
    .planOption {
      display:block;
      padding:18px;
      border:1px solid var(--line);
      border-radius:22px;
      background:var(--card);
      cursor:pointer;
    }
    .planOption input { margin-right:10px; transform:translateY(1px); }
    .planOption strong { font-size:18px; }
    .badge {
      display:inline-flex;
      padding:5px 10px;
      border-radius:999px;
      background:#ecf6ef;
      color:var(--forest);
      font-size:12px;
      font-weight:700;
      margin-bottom:10px;
    }
    ul { margin:10px 0 0; padding-left:18px; color:var(--muted); }
    form { display:grid; gap:14px; }
    .inputs { display:grid; gap:12px; grid-template-columns:repeat(auto-fit,minmax(220px,1fr)); }
    label { display:block; font-size:14px; font-weight:700; margin-bottom:6px; }
    input, textarea, button {
      width:100%;
      border-radius:16px;
      border:1px solid var(--line);
      padding:13px 15px;
      font:inherit;
      background:#fffdf8;
      color:var(--ink);
    }
    textarea { min-height:110px; resize:vertical; }
    button {
      border:none;
      background:linear-gradient(135deg, var(--accent), var(--accent-soft));
      color:white;
      font-weight:800;
      letter-spacing:0.02em;
      cursor:pointer;
      box-shadow:0 18px 36px rgba(203,75,22,0.22);
    }
    .meta { display:grid; gap:12px; }
    .infoList { display:grid; gap:10px; }
    .infoRow {
      display:flex;
      justify-content:space-between;
      gap:16px;
      padding:12px 14px;
      border-radius:16px;
      background:rgba(255,255,255,0.62);
      border:1px solid var(--line);
    }
    .warn {
      padding:14px 16px;
      border-radius:16px;
      background:#fff1ec;
      color:var(--bad);
      border:1px solid rgba(127,29,29,0.12);
      line-height:1.5;
    }
    @media (max-width: 860px) {
      .hero { grid-template-columns:1fr; }
    }
  </style>
</head>
<body>
  <div class="shell">
    <section class="hero">
      <div class="heroCard">
        <div class="eyebrow">VPN access · ручная модерация</div>
        <h1>Купить VPN и получить доступ сразу</h1>
        <p class="lead">
          После перевода на карту и загрузки скрина система сразу выдаёт код или профиль. Платёж остаётся на ручной проверке:
          если подтверждения оплаты нет, доступ отключается, а привязанные устройства попадают в блэклист.
        </p>
        <div class="grid">
          <div class="card">
            <div class="muted">Оплата</div>
            <div class="price">Перевод на карту</div>
            <div class="infoList" style="margin-top:14px;">
              <div class="infoRow"><span>Карта</span><strong>{{if .PaymentCardNumber}}{{.PaymentCardNumber}}{{else}}указать в конфиге{{end}}</strong></div>
              <div class="infoRow"><span>Получатель</span><strong>{{if .PaymentCardHolder}}{{.PaymentCardHolder}}{{else}}указать в конфиге{{end}}</strong></div>
              <div class="infoRow"><span>Банк</span><strong>{{if .PaymentCardBank}}{{.PaymentCardBank}}{{else}}указать в конфиге{{end}}</strong></div>
            </div>
          </div>
          <div class="card">
            <div class="muted">Что выдаётся</div>
            <ul>
              <li>код активации для Android / Windows, если выбран кодовый тариф;</li>
              <li>ссылка для Happ, vless:// и YAML-профиль, если выбран iOS / Windows тариф;</li>
              <li>ссылки на лаунчеры и кабинет заказа.</li>
            </ul>
          </div>
        </div>
      </div>
      <div class="card">
        <h2 style="margin-top:0;">Оформить заказ</h2>
        <form method="post" action="/order" enctype="multipart/form-data">
          <div class="planPicker">
            {{range $index, $plan := .Plans}}
            <label class="planOption">
              <div class="badge">{{$plan.Badge}}</div>
              <div>
                <input type="radio" name="plan_id" value="{{$plan.ID}}" {{if eq $index 0}}checked{{end}}>
                <strong>{{$plan.Name}}</strong>
              </div>
              <div class="price" style="font-size:28px; margin-top:8px;">{{formatMoney $plan.PriceMinor $plan.Currency}}</div>
              <div class="muted">{{$plan.Description}}</div>
              {{if $plan.Features}}
              <ul>
                {{range $plan.Features}}<li>{{.}}</li>{{end}}
              </ul>
              {{end}}
            </label>
            {{end}}
          </div>
          <div class="inputs">
            <div>
              <label for="customer_name">Имя</label>
              <input id="customer_name" name="customer_name" placeholder="Например, Ivan" required>
            </div>
            <div>
              <label for="contact">Контакт</label>
              <input id="contact" name="contact" placeholder="Telegram, email или телефон" required>
            </div>
            <div>
              <label for="device_label">Устройство</label>
              <input id="device_label" name="device_label" placeholder="Например, iPhone 15 / ПК дома">
            </div>
            <div>
              <label for="payment_screenshot">Скрин оплаты</label>
              <input id="payment_screenshot" name="payment_screenshot" type="file" accept=".png,.jpg,.jpeg,.webp" required>
            </div>
          </div>
          <div>
            <label for="note">Комментарий</label>
            <textarea id="note" name="note" placeholder="Можно указать удобный способ связи или детали по устройству"></textarea>
          </div>
          <div class="warn">
            Фальшивая оплата, чужой скрин или отменённый перевод приводят к отклонению заказа. Если к моменту отклонения доступ уже был активирован, аккаунт и привязанные устройства блокируются.
          </div>
          <button type="submit">Оплатил, выдать доступ</button>
        </form>
      </div>
    </section>
    <section class="grid" style="margin-top:18px;">
      <div class="card meta">
        <h3 style="margin:0;">Ссылки на клиенты</h3>
        <div class="infoList">
          {{if .AndroidLauncherURL}}<div class="infoRow"><span>Android</span><a href="{{.AndroidLauncherURL}}" target="_blank" rel="noreferrer">Скачать APK</a></div>{{end}}
          {{if .WindowsLauncherURL}}<div class="infoRow"><span>Windows</span><a href="{{.WindowsLauncherURL}}" target="_blank" rel="noreferrer">Скачать лаунчер</a></div>{{end}}
          {{if .HappDownloadURL}}<div class="infoRow"><span>iOS / Happ</span><a href="{{.HappDownloadURL}}" target="_blank" rel="noreferrer">Открыть Happ</a></div>{{end}}
        </div>
      </div>
      <div class="card meta">
        <h3 style="margin:0;">Где брать ссылку для Happ</h3>
        <p class="muted" style="margin:0;">
          Для тарифа iOS / Windows ссылка выдаётся сразу в кабинете заказа как поле <strong>«Ссылка для Happ»</strong>.
          Технически это публичный endpoint вида <code>/admin/client/subscription?client_uuid=...</code>.
        </p>
      </div>
      <div class="card meta">
        <h3 style="margin:0;">Поддержка</h3>
        <p class="muted" style="margin:0;">
          {{if .SupportLink}}Если нужен ручной разбор заказа или перенос доступа, используйте <a href="{{.SupportLink}}" target="_blank" rel="noreferrer">{{.SupportLink}}</a>.{{else}}Ссылку на поддержку можно добавить в конфиг support_link.{{end}}
        </p>
      </div>
    </section>
  </div>
</body>
</html>`

const orderTemplate = `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Заказ {{.Order.ID}}</title>
  <style>
    :root {
      --bg:#0f1115;
      --panel:#171b22;
      --card:#1d2330;
      --line:rgba(255,255,255,0.08);
      --ink:#f5f7fa;
      --muted:#a6afbd;
      --accent:#4ad3a8;
      --accent2:#ff8a3d;
      --bad:#ff7b7b;
      --shadow:0 22px 60px rgba(0,0,0,0.28);
    }
    * { box-sizing:border-box; }
    body {
      margin:0;
      background:
        radial-gradient(circle at top right, rgba(74,211,168,0.14), transparent 30%),
        radial-gradient(circle at left center, rgba(255,138,61,0.16), transparent 28%),
        linear-gradient(180deg, #0c1016 0%, #111723 100%);
      color:var(--ink);
      font-family:"Segoe UI Variable","Trebuchet MS","Segoe UI",sans-serif;
    }
    a { color:#8ce9c7; word-break:break-all; }
    code, pre { font-family:"Cascadia Code","Consolas",monospace; }
    .shell { max-width:1080px; margin:0 auto; padding:28px 18px 64px; }
    .stack { display:grid; gap:16px; }
    .card {
      background:linear-gradient(180deg, rgba(255,255,255,0.03), transparent), var(--card);
      border:1px solid var(--line);
      border-radius:24px;
      padding:22px;
      box-shadow:var(--shadow);
    }
    .hero {
      display:grid;
      gap:16px;
      grid-template-columns:1.1fr 0.9fr;
    }
    .pill {
      display:inline-flex;
      align-items:center;
      padding:7px 12px;
      border-radius:999px;
      font-size:12px;
      font-weight:800;
      letter-spacing:0.05em;
      text-transform:uppercase;
    }
    .pending { background:rgba(255,138,61,0.16); color:#ffbd8a; }
    .confirmed { background:rgba(74,211,168,0.16); color:#8ce9c7; }
    .rejected { background:rgba(255,123,123,0.16); color:#ffb2b2; }
    h1 { margin:14px 0 10px; font-size:clamp(28px,4vw,50px); letter-spacing:-0.04em; }
    .muted { color:var(--muted); }
    .grid { display:grid; gap:16px; grid-template-columns:repeat(auto-fit,minmax(230px,1fr)); }
    .row {
      display:flex;
      justify-content:space-between;
      gap:16px;
      padding:12px 0;
      border-bottom:1px solid var(--line);
    }
    .row:last-child { border-bottom:none; padding-bottom:0; }
    .bundle {
      border:1px solid var(--line);
      border-radius:18px;
      padding:16px;
      background:rgba(255,255,255,0.03);
    }
    .actions { display:flex; flex-wrap:wrap; gap:12px; }
    .button {
      display:inline-flex;
      align-items:center;
      justify-content:center;
      min-height:46px;
      padding:0 16px;
      border-radius:14px;
      background:linear-gradient(135deg, #39bf94, #2aa7d5);
      color:white;
      text-decoration:none;
      font-weight:800;
    }
    pre {
      margin:0;
      padding:14px;
      border-radius:16px;
      background:#11161f;
      color:#dfe8f3;
      overflow:auto;
      white-space:pre-wrap;
      word-break:break-word;
    }
    @media (max-width: 860px) {
      .hero { grid-template-columns:1fr; }
    }
  </style>
</head>
<body>
  <div class="shell stack">
    <section class="hero">
      <div class="card">
        <span class="pill {{statusClass .Order.Status}}">{{statusLabel .Order.Status}}</span>
        <h1>Заказ {{.Order.ID}}</h1>
        <p class="muted" style="margin:0;">
          Доступ выдан сразу. Статус оплаты пока проверяется модератором вручную.
          При отклонении заказ отключается, а уже привязанные устройства блокируются.
        </p>
        <div class="grid" style="margin-top:18px;">
          <div class="bundle">
            <div class="muted">Тариф</div>
            <strong>{{.Order.PlanName}}</strong>
            <div style="margin-top:8px;">{{formatMoney .Order.PriceMinor .Order.Currency}}</div>
          </div>
          <div class="bundle">
            <div class="muted">Контакт</div>
            <strong>{{.Order.Contact}}</strong>
            <div style="margin-top:8px;">{{if .Order.CustomerName}}{{.Order.CustomerName}}{{else}}без имени{{end}}</div>
          </div>
          <div class="bundle">
            <div class="muted">Создан</div>
            <strong>{{formatTime .Order.CreatedAt}}</strong>
            <div style="margin-top:8px;">{{if .Order.DeviceLabel}}{{.Order.DeviceLabel}}{{else}}устройство не указано{{end}}</div>
          </div>
        </div>
      </div>
      <div class="card">
        <h2 style="margin-top:0;">Ссылки на клиенты</h2>
        <div class="actions">
          {{if .AndroidLauncherURL}}<a class="button" href="{{.AndroidLauncherURL}}" target="_blank" rel="noreferrer">Android APK</a>{{end}}
          {{if .WindowsLauncherURL}}<a class="button" href="{{.WindowsLauncherURL}}" target="_blank" rel="noreferrer">Windows launcher</a>{{end}}
          {{if .HappDownloadURL}}<a class="button" href="{{.HappDownloadURL}}" target="_blank" rel="noreferrer">Happ для iOS</a>{{end}}
        </div>
        <p class="muted" style="margin:16px 0 0;">
          Этот кабинет можно открыть снова по этой же ссылке. Для Happ нужна строка <strong>«Ссылка для Happ»</strong> ниже.
        </p>
      </div>
    </section>

    <section class="card stack">
      <h2 style="margin:0;">Выданный доступ</h2>
      {{if .Order.InviteCode}}
      <div class="bundle">
        <div class="muted">Код активации</div>
        <pre>{{.Order.InviteCode}}</pre>
        <p class="muted" style="margin:10px 0 0;">Подходит для Android и Windows-лаунчера, если тариф выдает код.</p>
      </div>
      {{end}}

      {{if .Order.SubscriptionURL}}
      <div class="bundle">
        <div class="muted">Ссылка для Happ</div>
        <pre>{{.Order.SubscriptionURL}}</pre>
      </div>
      {{end}}

      {{if .Order.PrimaryVLESSURL}}
      <div class="bundle">
        <div class="muted">Основной vless://</div>
        <pre>{{.Order.PrimaryVLESSURL}}</pre>
      </div>
      {{end}}

      {{if .Order.VLESSURLs}}
      <div class="bundle">
        <div class="muted">Все VLESS-ссылки</div>
        <pre>{{joinLines .Order.VLESSURLs}}</pre>
      </div>
      {{end}}

      {{if .Order.ProfileYAMLs}}
      <div class="bundle">
        <div class="muted">YAML-профили для Windows-лаунчера</div>
        <div class="actions" style="margin-top:10px;">
          {{range $index, $_ := .Order.ProfileYAMLs}}
          <a class="button" href="/order/{{$.Order.ID}}/profiles/{{$index}}.yaml?token={{$.Order.AccessToken}}">Скачать профиль {{$index}}</a>
          {{end}}
        </div>
      </div>
      {{end}}

      {{if .Order.SubscriptionText}}
      <div class="bundle">
        <div class="muted">Текст подписки</div>
        <pre>{{.Order.SubscriptionText}}</pre>
      </div>
      {{end}}
    </section>

    <section class="card">
      <h2 style="margin-top:0;">Статус модерации</h2>
      <div class="row"><span class="muted">Текущий статус</span><strong>{{statusLabel .Order.Status}}</strong></div>
      {{if .Order.ReviewReason}}<div class="row"><span class="muted">Комментарий модератора</span><strong>{{.Order.ReviewReason}}</strong></div>{{end}}
      {{if .Order.ReviewedAt}}<div class="row"><span class="muted">Обновлено</span><strong>{{formatTimePtr .Order.ReviewedAt}}</strong></div>{{end}}
      {{if and (eq .Order.DeliveryMode "invite_code") (not .Order.SubscriptionURL)}}
      <p class="muted" style="margin:16px 0 0;">
        Для Happ ссылка сразу не выдается на кодовом тарифе. Она формируется только для готового профиля и находится в ответе сервера как
        <code>/admin/client/subscription?client_uuid=...</code>.
      </p>
      {{end}}
    </section>
  </div>
</body>
</html>`

const moderatorLoginTemplate = `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Модератор</title>
  <style>
    body {
      margin:0;
      min-height:100vh;
      display:grid;
      place-items:center;
      background:linear-gradient(180deg, #10151d, #0d1118);
      color:#f5f7fa;
      font-family:"Segoe UI Variable","Trebuchet MS","Segoe UI",sans-serif;
    }
    form {
      width:min(420px, calc(100vw - 28px));
      padding:24px;
      border-radius:24px;
      border:1px solid rgba(255,255,255,0.08);
      background:#171c25;
      box-shadow:0 18px 50px rgba(0,0,0,0.25);
    }
    input, button {
      width:100%;
      border-radius:14px;
      border:1px solid rgba(255,255,255,0.1);
      padding:13px 14px;
      font:inherit;
      margin-top:12px;
      background:#0e131a;
      color:#f5f7fa;
    }
    button {
      border:none;
      background:linear-gradient(135deg, #2aa7d5, #39bf94);
      font-weight:800;
      cursor:pointer;
    }
    .notice { color:#ffb2b2; min-height:20px; }
  </style>
</head>
<body>
  <form method="post" action="/moderator/login">
    <h1 style="margin:0 0 8px;">Вход модератора</h1>
    <div class="notice">{{.Notice}}</div>
    <input name="token" type="password" placeholder="Токен доступа" autofocus>
    <button type="submit">Войти</button>
  </form>
</body>
</html>`

const moderatorOrdersTemplate = `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Заказы</title>
  <style>
    :root {
      --bg:#0f1218;
      --panel:#171c25;
      --line:rgba(255,255,255,0.08);
      --ink:#eff4fb;
      --muted:#98a2b3;
      --green:#8ce9c7;
      --orange:#ffbd8a;
      --red:#ffb2b2;
    }
    * { box-sizing:border-box; }
    body { margin:0; background:var(--bg); color:var(--ink); font-family:"Segoe UI Variable","Trebuchet MS","Segoe UI",sans-serif; }
    a { color:#9fe3ff; text-decoration:none; }
    .shell { max-width:1220px; margin:0 auto; padding:24px 18px 48px; }
    .card { background:var(--panel); border:1px solid var(--line); border-radius:22px; padding:18px; }
    table { width:100%; border-collapse:collapse; }
    th, td { text-align:left; padding:12px 10px; border-bottom:1px solid var(--line); vertical-align:top; }
    th { color:var(--muted); font-size:13px; text-transform:uppercase; letter-spacing:0.04em; }
    .pill { display:inline-flex; padding:5px 10px; border-radius:999px; font-size:12px; font-weight:800; text-transform:uppercase; }
    .pending_review { background:rgba(255,189,138,0.14); color:var(--orange); }
    .confirmed { background:rgba(140,233,199,0.14); color:var(--green); }
    .rejected { background:rgba(255,178,178,0.14); color:var(--red); }
    .actions { display:flex; flex-wrap:wrap; gap:8px; }
    button {
      padding:9px 12px;
      border-radius:12px;
      border:none;
      cursor:pointer;
      font:inherit;
      font-weight:700;
    }
    .approve { background:#1d6f57; color:white; }
    .reject { background:#7f1d1d; color:white; }
    input[type="text"] {
      width:100%;
      padding:10px 12px;
      border-radius:12px;
      border:1px solid var(--line);
      background:#10151c;
      color:var(--ink);
    }
  </style>
</head>
<body>
  <div class="shell">
    <div class="card">
      <div style="display:flex; justify-content:space-between; gap:16px; align-items:center; flex-wrap:wrap;">
        <div>
          <h1 style="margin:0;">Очередь заказов</h1>
          <div class="muted">Клиенты уже получили доступ. Здесь подтверждается или отклоняется оплата.</div>
        </div>
        <a href="/moderator/logout">Выйти</a>
      </div>
      <table style="margin-top:18px;">
        <thead>
          <tr>
            <th>ID</th>
            <th>Тариф</th>
            <th>Контакт</th>
            <th>Доступ</th>
            <th>Статус</th>
            <th>Скрин</th>
            <th>Действия</th>
          </tr>
        </thead>
        <tbody>
          {{range .Orders}}
          <tr>
            <td><a href="/moderator/orders/{{.ID}}">{{.ID}}</a><div class="muted">{{formatTime .CreatedAt}}</div></td>
            <td>{{.PlanName}}<div class="muted">{{formatMoney .PriceMinor .Currency}}</div></td>
            <td>{{.Contact}}<div class="muted">{{.CustomerName}}</div></td>
            <td>{{if .ClientID}}client {{.ClientID}}{{else}}invite {{.InviteCode}}{{end}}</td>
            <td><span class="pill {{.Status}}">{{statusLabel .Status}}</span></td>
            <td>{{if .ScreenshotFile}}<a href="/moderator/orders/{{.ID}}/screenshot" target="_blank" rel="noreferrer">Открыть</a>{{end}}</td>
            <td>
              <div class="actions">
                <form method="post" action="/moderator/orders/{{.ID}}/confirm">
                  <button class="approve" type="submit">Подтвердить</button>
                </form>
              </div>
              <form method="post" action="/moderator/orders/{{.ID}}/reject" style="margin-top:8px;">
                <input type="text" name="reason" placeholder="Причина отклонения" value="{{.ReviewReason}}">
                <button class="reject" type="submit" style="margin-top:8px;">Отклонить и заблокировать</button>
              </form>
            </td>
          </tr>
          {{else}}
          <tr><td colspan="7">Заказов пока нет.</td></tr>
          {{end}}
        </tbody>
      </table>
    </div>
  </div>
</body>
</html>`
