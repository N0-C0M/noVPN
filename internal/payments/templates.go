package payments

const landingTemplate = `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.BrandName}} · VPN на ваших условиях</title>
  <style>
    :root {
      --bg:#f2efe8;
      --sand:#e6dcc8;
      --ink:#0f1720;
      --muted:#56616b;
      --card:#fffaf2;
      --line:rgba(15,23,32,0.09);
      --teal:#0f766e;
      --teal-soft:#7dd3c7;
      --orange:#dd6b20;
      --orange-soft:#ffb36b;
      --slate:#1f2937;
      --shadow:0 26px 70px rgba(20,31,45,0.12);
    }
    * { box-sizing:border-box; }
    body {
      margin:0;
      color:var(--ink);
      background:
        radial-gradient(circle at 8% 12%, rgba(221,107,32,0.20), transparent 26%),
        radial-gradient(circle at 92% 0%, rgba(15,118,110,0.18), transparent 24%),
        linear-gradient(180deg, #f8f5ef 0%, #eee6d7 100%);
      font-family:"Aptos","Segoe UI Variable","Trebuchet MS",sans-serif;
    }
    a { color:var(--teal); }
    .shell { max-width:1240px; margin:0 auto; padding:24px 18px 64px; }
    .hero {
      display:grid;
      grid-template-columns:1.1fr 0.9fr;
      gap:18px;
      align-items:start;
    }
    .panel {
      background:linear-gradient(180deg, rgba(255,255,255,0.78), rgba(255,255,255,0.92));
      border:1px solid var(--line);
      border-radius:30px;
      box-shadow:var(--shadow);
      backdrop-filter:blur(12px);
    }
    .heroCard { padding:28px; }
    .eyebrow {
      display:inline-flex;
      align-items:center;
      gap:8px;
      padding:8px 12px;
      border-radius:999px;
      background:#fff0de;
      color:var(--orange);
      font-size:12px;
      font-weight:800;
      letter-spacing:0.08em;
      text-transform:uppercase;
    }
    h1 {
      margin:16px 0 10px;
      font-size:clamp(38px, 6vw, 72px);
      line-height:0.94;
      letter-spacing:-0.05em;
      max-width:11ch;
    }
    .lead {
      margin:0;
      color:var(--muted);
      font-size:18px;
      line-height:1.7;
      max-width:48rem;
    }
    .kpis, .grid {
      display:grid;
      gap:14px;
      grid-template-columns:repeat(auto-fit,minmax(220px,1fr));
    }
    .kpis { margin-top:22px; }
    .stat, .tile, .checkout, .card {
      background:var(--card);
      border:1px solid var(--line);
      border-radius:24px;
    }
    .stat, .tile, .card { padding:18px; }
    .value { font-size:32px; font-weight:900; letter-spacing:-0.04em; }
    .muted { color:var(--muted); }
    .builder { padding:22px; }
    .builder h2 { margin:0 0 14px; font-size:24px; letter-spacing:-0.03em; }
    .sectionLabel {
      display:block;
      margin-bottom:8px;
      color:var(--muted);
      font-size:13px;
      font-weight:800;
      letter-spacing:0.08em;
      text-transform:uppercase;
    }
    .months {
      display:grid;
      gap:10px;
      grid-template-columns:repeat(2,minmax(0,1fr));
    }
    .monthBtn {
      width:100%;
      padding:14px 14px;
      border-radius:18px;
      border:1px solid var(--line);
      background:#fff;
      color:var(--ink);
      font:inherit;
      font-weight:800;
      text-align:left;
      cursor:pointer;
    }
    .monthBtn span { display:block; }
    .monthBtn .sub { margin-top:4px; color:var(--muted); font-size:13px; font-weight:700; }
    .monthBtn.active {
      border-color:rgba(15,118,110,0.28);
      background:linear-gradient(135deg, rgba(15,118,110,0.12), rgba(125,211,199,0.12));
      box-shadow:inset 0 0 0 1px rgba(15,118,110,0.10);
    }
    .rangeWrap {
      display:grid;
      gap:10px;
      margin-bottom:14px;
    }
    .devicesHead {
      display:flex;
      align-items:center;
      justify-content:space-between;
      gap:12px;
    }
    .bigNumber {
      font-size:40px;
      font-weight:900;
      letter-spacing:-0.05em;
    }
    input[type="range"] {
      width:100%;
      accent-color:var(--teal);
    }
    .summary {
      margin-top:18px;
      padding:18px;
      border-radius:24px;
      background:linear-gradient(135deg, #0f766e, #155e75);
      color:#f4fffd;
      box-shadow:0 22px 50px rgba(15,118,110,0.22);
    }
    .summaryPrice {
      font-size:40px;
      font-weight:900;
      letter-spacing:-0.05em;
    }
    .rows { display:grid; gap:10px; margin-top:14px; }
    .row {
      display:flex;
      justify-content:space-between;
      gap:16px;
      padding:12px 0;
      border-bottom:1px solid rgba(255,255,255,0.12);
    }
    .row:last-child { border-bottom:none; padding-bottom:0; }
    form { display:grid; gap:14px; }
    .fields {
      display:grid;
      gap:12px;
      grid-template-columns:repeat(auto-fit,minmax(220px,1fr));
    }
    input, textarea, button {
      width:100%;
      border-radius:18px;
      border:1px solid var(--line);
      padding:13px 14px;
      font:inherit;
      background:#fffefb;
      color:var(--ink);
    }
    textarea { min-height:110px; resize:vertical; }
    button[type="submit"] {
      border:none;
      background:linear-gradient(135deg, var(--orange), var(--orange-soft));
      color:white;
      font-weight:900;
      letter-spacing:0.02em;
      cursor:pointer;
      box-shadow:0 20px 40px rgba(221,107,32,0.24);
    }
    .notice {
      padding:14px 16px;
      border-radius:18px;
      background:#eefaf8;
      border:1px solid rgba(15,118,110,0.12);
      color:#0f4e49;
      line-height:1.6;
    }
    .heroFoot {
      margin-top:18px;
      display:grid;
      gap:14px;
      grid-template-columns:repeat(auto-fit,minmax(220px,1fr));
    }
    .support {
      margin-top:18px;
      display:grid;
      gap:14px;
      grid-template-columns:repeat(auto-fit,minmax(240px,1fr));
    }
    .portalForm {
      display:flex;
      gap:10px;
      flex-wrap:wrap;
      align-items:center;
    }
    .portalForm input { flex:1 1 260px; }
    .portalForm button {
      width:auto;
      min-width:200px;
      background:linear-gradient(135deg, #0f766e, #38b2ac);
      box-shadow:none;
    }
    @media (max-width: 920px) {
      .hero { grid-template-columns:1fr; }
    }
  </style>
</head>
<body>
  <div class="shell">
    <section class="hero">
      <div class="panel heroCard">
        <div class="eyebrow">Частный VPN-сервис · один кабинет</div>
        <h1>{{.BrandName}} без шаблонных тарифов</h1>
        <p class="lead">{{.Pricing.ProductDescription}}</p>
        <div class="kpis">
          <div class="stat">
            <div class="muted">Оплата</div>
            <div class="value">СБП</div>
            <div class="muted">Интеграцию можно подключить позже без переделки витрины.</div>
          </div>
          <div class="stat">
            <div class="muted">Ключи</div>
            <div class="value">Happ + ключи</div>
            <div class="muted">На каждое устройство выпускается отдельный subscription URL.</div>
          </div>
          <div class="stat">
            <div class="muted">Кабинет</div>
            <div class="value">Ключ кабинета</div>
            <div class="muted">Один ключ от сайта для истории заказов, копирования ключей и продления.</div>
          </div>
        </div>
        <div class="heroFoot">
          <div class="tile">
            <strong>{{.Pricing.AccountPortalHeadline}}</strong>
            <p class="muted" style="margin:10px 0 0;">{{.Pricing.AccountPortalSubtext}}</p>
          </div>
          <div class="tile">
            <strong>Что входит</strong>
            <ul class="muted" style="margin:10px 0 0; padding-left:18px;">
              {{range .Pricing.Features}}<li>{{.}}</li>{{end}}
            </ul>
          </div>
        </div>
      </div>

      <div class="panel builder">
        <h2>Собрать тариф</h2>
        {{if .SiteKeyNotice}}<div class="notice" style="margin-bottom:14px;">{{.SiteKeyNotice}}</div>{{end}}
        <form method="post" action="/order">
          <input type="hidden" name="account_id" value="{{.AccountID}}">
          <input type="hidden" name="account_token" value="{{.AccountToken}}">
          <input type="hidden" name="device_count" id="device_count" value="{{.DefaultQuote.DeviceCount}}">
          <input type="hidden" name="months" id="months" value="{{.DefaultQuote.Months}}">

          <div class="rangeWrap">
            <div class="devicesHead">
              <div>
                <span class="sectionLabel">Количество устройств</span>
                <div class="muted">От {{.Pricing.MinDevices}} до {{.Pricing.MaxDevices}} устройств в одном заказе.</div>
              </div>
              <div class="bigNumber" id="deviceCountLabel">{{.DefaultQuote.DeviceCount}}</div>
            </div>
            <input id="deviceRange" type="range" min="{{.Pricing.MinDevices}}" max="{{.Pricing.MaxDevices}}" value="{{.DefaultQuote.DeviceCount}}">
          </div>

          <div>
            <span class="sectionLabel">Срок подписки</span>
            <div class="months" id="monthsGrid">
              {{range .Pricing.MonthOptions}}
              <button class="monthBtn" type="button" data-months="{{.Months}}" data-discount="{{.DiscountPercent}}">
                <span>{{.Label}}</span>
                <span class="sub">{{if gt .DiscountPercent 0}}Скидка {{.DiscountPercent}}%{{else}}Без скидки{{end}}</span>
              </button>
              {{end}}
            </div>
          </div>

          <div class="summary">
            <div class="muted" style="color:rgba(244,255,253,0.78);">Итого по заказу</div>
            <div class="summaryPrice" id="totalPrice">{{formatMoney .DefaultQuote.TotalMinor .DefaultQuote.Currency}}</div>
            <div id="summaryLine" style="font-size:15px; line-height:1.6;">{{.DefaultQuote.DeviceCount}} устройство · {{.DefaultQuote.Months}} мес. · скидка {{.DefaultQuote.DiscountPercent}}%</div>
            <div class="rows">
              <div class="row"><span>База</span><strong id="basePrice">{{formatMoney .DefaultQuote.SubtotalMinor .DefaultQuote.Currency}}</strong></div>
              <div class="row"><span>Скидка</span><strong id="discountPrice">{{formatMoney .DefaultQuote.DiscountMinor .DefaultQuote.Currency}}</strong></div>
            </div>
          </div>

          <div class="fields">
            <div>
              <label class="sectionLabel" for="customer_name">Имя</label>
              <input id="customer_name" name="customer_name" value="{{.CustomerName}}" placeholder="Например, Ivan" required>
            </div>
            <div>
              <label class="sectionLabel" for="contact">Контакт</label>
              <input id="contact" name="contact" value="{{.Contact}}" placeholder="Telegram, email или телефон" required>
            </div>
            <div>
              <label class="sectionLabel" for="device_label">Подпись для слотов</label>
              <input id="device_label" name="device_label" value="{{.DeviceLabel}}" placeholder="Например, Family / Office / Personal">
            </div>
            <div>
              <label class="sectionLabel" for="promo_code">Промокод</label>
              <input id="promo_code" name="promo_code" value="{{.PromoCode}}" placeholder="Если есть скидка или тестовый код">
            </div>
          </div>
          <div>
            <label class="sectionLabel" for="note">Комментарий</label>
            <textarea id="note" name="note" placeholder="Например, какие устройства подключать в первую очередь">{{.Note}}</textarea>
          </div>
          <div class="notice">{{.Pricing.SBPPaymentNotice}}</div>
          <button type="submit">Перейти к оплате и выпуску ключей</button>
        </form>
      </div>
    </section>

    <section class="support">
      <div class="card">
        <strong>Уже есть ключ кабинета?</strong>
        <p class="muted" style="margin:10px 0 12px;">Открой кабинет, чтобы скопировать текущие ключи, посмотреть историю заказов и оформить продление.</p>
        <form class="portalForm" method="post" action="/cabinet/open">
          <input name="site_key" placeholder="Вставьте ключ от сайта" required>
          <button type="submit">Открыть кабинет</button>
        </form>
      </div>
      <div class="card">
        <strong>Клиенты</strong>
        <div class="grid" style="margin-top:12px;">
          {{if .HappDownloadURL}}<div class="tile"><div class="muted">Happ</div><a href="{{.HappDownloadURL}}" target="_blank" rel="noreferrer">Открыть / скачать</a></div>{{end}}
          {{if .AndroidLauncherURL}}<div class="tile"><div class="muted">Android APK</div><a href="{{.AndroidLauncherURL}}" target="_blank" rel="noreferrer">Скачать</a></div>{{end}}
          {{if .WindowsLauncherURL}}<div class="tile"><div class="muted">ПК-клиент</div><a href="{{.WindowsLauncherURL}}" target="_blank" rel="noreferrer">Скачать</a></div>{{end}}
        </div>
      </div>
      <div class="card">
        <strong>Поддержка</strong>
        <p class="muted" style="margin:10px 0 0;">
          {{if .SupportLink}}Нужна ручная помощь или перенос доступа: <a href="{{.SupportLink}}" target="_blank" rel="noreferrer">{{.SupportLink}}</a>.{{else}}Ссылку на поддержку можно добавить через ` + "`support_link`" + ` в конфиге pay-service.{{end}}
        </p>
      </div>
    </section>
  </div>

  <script>
    (() => {
      const pricing = {
        baseMonthly: {{.Pricing.BaseMonthlyPriceMinor}},
        currency: {{printf "%q" .Pricing.Currency}},
        options: {{toJSON .Pricing.MonthOptions}}
      };
      const currency = pricing.currency || "RUB";
      const range = document.getElementById("deviceRange");
      const deviceCountInput = document.getElementById("device_count");
      const monthsInput = document.getElementById("months");
      const deviceCountLabel = document.getElementById("deviceCountLabel");
      const totalPrice = document.getElementById("totalPrice");
      const basePrice = document.getElementById("basePrice");
      const discountPrice = document.getElementById("discountPrice");
      const summaryLine = document.getElementById("summaryLine");
      const buttons = Array.from(document.querySelectorAll(".monthBtn"));

      function money(value) {
        if ((currency || "").toUpperCase() === "RUB") return value + " ₽";
        return value + " " + currency;
      }

      function selectedOption() {
        const months = Number(monthsInput.value || 0);
        return pricing.options.find(item => item.months === months) || pricing.options[0];
      }

      function render() {
        const devices = Number(range.value || 1);
        const option = selectedOption();
        const subtotal = pricing.baseMonthly * devices * option.months;
        const discount = Math.floor(subtotal * option.discount_percent / 100);
        const total = subtotal - discount;

        deviceCountInput.value = String(devices);
        deviceCountLabel.textContent = String(devices);
        totalPrice.textContent = money(total);
        basePrice.textContent = money(subtotal);
        discountPrice.textContent = money(discount);
        summaryLine.textContent = devices + " устройств · " + option.months + " мес. · скидка " + option.discount_percent + "%";
        buttons.forEach(btn => {
          btn.classList.toggle("active", Number(btn.dataset.months) === option.months);
        });
      }

      buttons.forEach(btn => {
        btn.addEventListener("click", () => {
          monthsInput.value = btn.dataset.months;
          render();
        });
      });
      range.addEventListener("input", render);
      render();
    })();
  </script>
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
      --bg:#0c121a;
      --ink:#eef5ff;
      --muted:#9aa6b2;
      --panel:#101924;
      --card:#162230;
      --line:rgba(255,255,255,0.08);
      --teal:#38b2ac;
      --green:#0f766e;
      --orange:#ff9a4a;
      --red:#ff8a8a;
      --shadow:0 26px 80px rgba(0,0,0,0.30);
    }
    * { box-sizing:border-box; }
    body {
      margin:0;
      color:var(--ink);
      background:
        radial-gradient(circle at 10% 10%, rgba(56,178,172,0.16), transparent 24%),
        radial-gradient(circle at 90% 0%, rgba(255,154,74,0.18), transparent 26%),
        linear-gradient(180deg, #081019 0%, #0f1622 100%);
      font-family:"Aptos","Segoe UI Variable","Trebuchet MS",sans-serif;
    }
    a { color:#9ce7de; word-break:break-all; }
    code, pre { font-family:"Cascadia Code","Consolas",monospace; }
    .shell { max-width:1180px; margin:0 auto; padding:26px 18px 60px; display:grid; gap:16px; }
    .hero { display:grid; gap:16px; grid-template-columns:1.05fr 0.95fr; }
    .panel {
      background:linear-gradient(180deg, rgba(255,255,255,0.03), transparent), var(--panel);
      border:1px solid var(--line);
      border-radius:28px;
      box-shadow:var(--shadow);
      padding:22px;
    }
    .pill {
      display:inline-flex;
      align-items:center;
      padding:7px 12px;
      border-radius:999px;
      font-size:12px;
      font-weight:900;
      letter-spacing:0.08em;
      text-transform:uppercase;
    }
    .awaiting_payment { background:rgba(255,154,74,0.14); color:#ffd1a7; }
    .provisioning { background:rgba(56,178,172,0.14); color:#a9efe7; }
    .active { background:rgba(15,118,110,0.18); color:#9ce7de; }
    .cancelled { background:rgba(255,138,138,0.14); color:#ffbbbb; }
    h1 { margin:16px 0 10px; font-size:clamp(30px,4vw,56px); letter-spacing:-0.05em; }
    .muted { color:var(--muted); }
    .grid { display:grid; gap:14px; grid-template-columns:repeat(auto-fit,minmax(220px,1fr)); }
    .tile, .card {
      background:var(--card);
      border:1px solid var(--line);
      border-radius:22px;
      padding:18px;
    }
    .price { font-size:38px; font-weight:900; letter-spacing:-0.05em; }
    .rows { display:grid; gap:10px; margin-top:14px; }
    .row {
      display:flex;
      justify-content:space-between;
      gap:12px;
      padding:12px 0;
      border-bottom:1px solid var(--line);
    }
    .row:last-child { border-bottom:none; padding-bottom:0; }
    .cta {
      display:inline-flex;
      align-items:center;
      justify-content:center;
      min-height:48px;
      padding:0 18px;
      border:none;
      border-radius:16px;
      background:linear-gradient(135deg, var(--orange), #ffb768);
      color:#fff;
      font:inherit;
      font-weight:900;
      cursor:pointer;
      text-decoration:none;
    }
    .cta.secondary {
      background:linear-gradient(135deg, #0f766e, #38b2ac);
    }
    .qr {
      min-height:220px;
      display:grid;
      place-items:center;
      border-radius:24px;
      border:1px dashed rgba(255,255,255,0.18);
      background:
        linear-gradient(135deg, rgba(255,255,255,0.03), rgba(255,255,255,0.01)),
        repeating-linear-gradient(45deg, rgba(255,255,255,0.02), rgba(255,255,255,0.02) 10px, transparent 10px, transparent 20px);
      text-align:center;
      padding:18px;
    }
    pre {
      margin:10px 0 0;
      padding:14px;
      border-radius:16px;
      background:#0d151f;
      overflow:auto;
      white-space:pre-wrap;
      word-break:break-word;
    }
    .keyGrid { display:grid; gap:14px; grid-template-columns:repeat(auto-fit,minmax(280px,1fr)); }
    .notice {
      padding:14px 16px;
      border-radius:18px;
      background:rgba(255,138,138,0.12);
      border:1px solid rgba(255,138,138,0.12);
      color:#ffd2d2;
    }
    @media (max-width: 920px) {
      .hero { grid-template-columns:1fr; }
    }
  </style>
</head>
<body>
  <div class="shell">
    <section class="hero">
      <div class="panel">
        <span class="pill {{statusClass .Order.Status}}">{{statusLabel .Order.Status}}</span>
        <h1>{{.BrandName}} · заказ {{.Order.ID}}</h1>
        <p class="muted" style="margin:0;">После подтверждения сервис выпускает отдельный ключ на каждый слот устройства и привязывает заказ к вашему ключу кабинета.</p>
        <div class="grid" style="margin-top:18px;">
          <div class="tile">
            <div class="muted">Собранный тариф</div>
            <strong>{{.Order.PlanName}}</strong>
            <div style="margin-top:8px;">{{.Order.DeviceCount}} устройств · {{.Order.Months}} мес.</div>
          </div>
          <div class="tile">
            <div class="muted">Контакт</div>
            <strong>{{.Order.Contact}}</strong>
            <div style="margin-top:8px;">{{.Order.CustomerName}}</div>
          </div>
          <div class="tile">
            <div class="muted">Ключ кабинета</div>
            <strong>{{.Order.AccountToken}}</strong>
            <div style="margin-top:8px;"><a href="{{.AccountURL}}">Открыть кабинет</a></div>
          </div>
        </div>
      </div>

      <div class="panel">
        {{if eq .Order.Status "active"}}
        <div class="muted">{{if and (eq .Order.PriceMinor 0) .Order.PromoCode}}Оплата не требуется{{else}}К оплате{{end}}</div>
        <div class="price">{{formatMoney .Order.PriceMinor .Order.Currency}}</div>
        <p class="muted" style="line-height:1.7;">{{if and (eq .Order.PriceMinor 0) .Order.PromoCode}}Заказ был активирован без СБП: промокод полностью покрыл стоимость. Ключи уже выпущены и доступны ниже.{{else}}Оплата зафиксирована. Ключи уже выпущены и доступны ниже. Для следующих покупок и продления используйте ваш кабинет.{{end}}</p>
        <a class="cta secondary" href="{{.AccountURL}}">Перейти в кабинет</a>
        {{else}}
        <div class="muted">{{if and (eq .Order.PriceMinor 0) .Order.PromoCode}}Активация по промокоду{{else}}Этап оплаты{{end}}</div>
        <div class="price">{{formatMoney .Order.PriceMinor .Order.Currency}}</div>
        {{if and (eq .Order.PriceMinor 0) .Order.PromoCode}}
        <p class="muted" style="line-height:1.7;">Промокод полностью покрыл стоимость заказа. СБП для этого заказа не требуется. Если автоматический выпуск ключей не завершился, нажмите кнопку ниже для повторной активации.</p>
        {{else}}
        <p class="muted" style="line-height:1.7;">СБП будет подключен реальным провайдером позже. Пока эта страница работает как checkout-заглушка: после нажатия кнопки ниже доступ выдается сразу.</p>
        <div class="qr">
          <div>
            <strong>{{if .SBPCardHolder}}{{.SBPCardHolder}}{{else}}Получатель СБП{{end}}</strong>
            <div class="muted" style="margin-top:8px;">{{if .SBPCardBank}}{{.SBPCardBank}}{{else}}Ваш банк по СБП{{end}}</div>
            <div style="margin-top:12px;">{{if .SBPCardNumber}}{{.SBPCardNumber}}{{else}}Подключите реальные реквизиты позже{{end}}</div>
          </div>
        </div>
        {{end}}
        {{if .Order.ProvisionError}}<div class="notice" style="margin-top:14px;">Последняя попытка выпуска ключей завершилась ошибкой: {{.Order.ProvisionError}}</div>{{end}}
        <form method="post" action="/order/{{.Order.ID}}/activate?token={{.Order.AccessToken}}" style="margin-top:14px;">
          <button class="cta" type="submit">{{if eq .Order.Status "provisioning"}}Выпуск уже идет{{else if and (eq .Order.PriceMinor 0) .Order.PromoCode}}Выдать ключи без оплаты{{else}}Я оплатил, выдать ключи{{end}}</button>
        </form>
        {{end}}
      </div>
    </section>

    <section class="panel">
      <h2 style="margin:0 0 12px; font-size:24px; letter-spacing:-0.03em;">Финальная конфигурация заказа</h2>
      <div class="rows">
        <div class="row"><span class="muted">База</span><strong>{{formatMoney .Order.SubtotalMinor .Order.Currency}}</strong></div>
        <div class="row"><span class="muted">Скидка за срок</span><strong>{{formatMoney .Order.DiscountMinor .Order.Currency}} ({{.Order.DiscountPercent}}%)</strong></div>
        {{if .Order.PromoCode}}<div class="row"><span class="muted">Промокод</span><strong>{{.Order.PromoCode}}{{if .Order.PromoName}} · {{.Order.PromoName}}{{end}}</strong></div>{{end}}
        {{if gt .Order.PromoDiscountMinor 0}}<div class="row"><span class="muted">Скидка по промокоду</span><strong>{{formatMoney .Order.PromoDiscountMinor .Order.Currency}} ({{.Order.PromoDiscountPercent}}%)</strong></div>{{end}}
        <div class="row"><span class="muted">Итого</span><strong>{{formatMoney .Order.PriceMinor .Order.Currency}}</strong></div>
        <div class="row"><span class="muted">Срок доступа</span><strong>{{.Order.Months}} мес. / {{.Order.DurationDays}} дней</strong></div>
        <div class="row"><span class="muted">Создан</span><strong>{{formatTime .Order.CreatedAt}}</strong></div>
        {{if .Order.ActivatedAt}}<div class="row"><span class="muted">Активирован</span><strong>{{formatTimePtr .Order.ActivatedAt}}</strong></div>{{end}}
      </div>
    </section>

    {{if .Order.AccessKeys}}
    <section class="panel">
      <h2 style="margin:0 0 14px; font-size:24px; letter-spacing:-0.03em;">Выданные ключи</h2>
      <div class="keyGrid">
        {{range .Order.AccessKeys}}
        <div class="card">
          <strong>Слот {{.SlotNumber}} · {{.Label}}</strong>
          <div class="muted" style="margin-top:8px;">ID клиента: {{.ClientID}}</div>
          {{if .SubscriptionURL}}<pre>{{.SubscriptionURL}}</pre>{{end}}
          {{if .PrimaryVLESSURL}}<div class="muted" style="margin-top:10px;">Резервный VLESS</div><pre>{{.PrimaryVLESSURL}}</pre>{{end}}
        </div>
        {{end}}
      </div>
    </section>
    {{end}}

    <section class="panel">
      <h2 style="margin:0 0 14px; font-size:24px; letter-spacing:-0.03em;">Клиенты и кабинет</h2>
      <div class="grid">
        <div class="tile">
          <div class="muted">Happ</div>
          {{if .HappURL}}<a href="{{.HappURL}}" target="_blank" rel="noreferrer">Открыть / скачать</a>{{else}}Ссылка не задана{{end}}
        </div>
        <div class="tile">
          <div class="muted">Android APK</div>
          {{if .AndroidURL}}<a href="{{.AndroidURL}}" target="_blank" rel="noreferrer">Скачать</a>{{else}}Ссылка не задана{{end}}
        </div>
        <div class="tile">
          <div class="muted">ПК-клиент</div>
          {{if .WindowsURL}}<a href="{{.WindowsURL}}" target="_blank" rel="noreferrer">Скачать</a>{{else}}Ссылка не задана{{end}}
        </div>
      </div>
    </section>
  </div>
</body>
</html>`

const accountTemplate = `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.BrandName}} · кабинет</title>
  <style>
    :root {
      --bg:#f3f5f7;
      --ink:#13202b;
      --muted:#5b6875;
      --panel:#ffffff;
      --line:rgba(19,32,43,0.08);
      --teal:#0f766e;
      --orange:#dd6b20;
      --slate:#223040;
      --shadow:0 24px 70px rgba(19,32,43,0.10);
    }
    * { box-sizing:border-box; }
    body {
      margin:0;
      color:var(--ink);
      background:
        radial-gradient(circle at top left, rgba(15,118,110,0.10), transparent 22%),
        radial-gradient(circle at top right, rgba(221,107,32,0.12), transparent 24%),
        linear-gradient(180deg, #f7f9fb 0%, #eef2f5 100%);
      font-family:"Aptos","Segoe UI Variable","Trebuchet MS",sans-serif;
    }
    a { color:var(--teal); word-break:break-all; }
    code, pre { font-family:"Cascadia Code","Consolas",monospace; }
    .shell { max-width:1240px; margin:0 auto; padding:24px 18px 64px; display:grid; gap:16px; }
    .hero, .grid { display:grid; gap:16px; }
    .hero { grid-template-columns:1.05fr 0.95fr; }
    .panel, .card {
      background:var(--panel);
      border:1px solid var(--line);
      border-radius:28px;
      box-shadow:var(--shadow);
    }
    .panel { padding:24px; }
    .card { padding:18px; }
    h1 { margin:12px 0 10px; font-size:clamp(32px,4.4vw,58px); letter-spacing:-0.05em; }
    .muted { color:var(--muted); }
    .siteKey {
      padding:16px 18px;
      border-radius:20px;
      background:linear-gradient(135deg, rgba(15,118,110,0.08), rgba(221,107,32,0.08));
      border:1px solid rgba(15,118,110,0.10);
    }
    .kpiGrid, .keyGrid, .orderGrid {
      display:grid;
      gap:14px;
      grid-template-columns:repeat(auto-fit,minmax(240px,1fr));
    }
    .big { font-size:34px; font-weight:900; letter-spacing:-0.05em; }
    pre {
      margin:10px 0 0;
      padding:14px;
      border-radius:16px;
      background:#edf4f3;
      overflow:auto;
      white-space:pre-wrap;
      word-break:break-word;
    }
    .builder {
      padding:22px;
      border-radius:24px;
      background:#fbfcfd;
      border:1px solid var(--line);
    }
    .months {
      display:grid;
      gap:10px;
      grid-template-columns:repeat(2,minmax(0,1fr));
      margin-top:10px;
    }
    .monthBtn {
      width:100%;
      padding:14px;
      border-radius:18px;
      border:1px solid var(--line);
      background:#fff;
      color:var(--ink);
      font:inherit;
      font-weight:800;
      text-align:left;
      cursor:pointer;
    }
    .monthBtn .sub { display:block; margin-top:6px; color:var(--muted); font-size:13px; }
    .monthBtn.active {
      border-color:rgba(15,118,110,0.22);
      background:linear-gradient(135deg, rgba(15,118,110,0.10), rgba(15,118,110,0.03));
    }
    input, textarea, button {
      width:100%;
      border-radius:18px;
      border:1px solid var(--line);
      padding:13px 14px;
      font:inherit;
      background:#fff;
      color:var(--ink);
    }
    textarea { min-height:100px; resize:vertical; }
    button[type="submit"] {
      border:none;
      background:linear-gradient(135deg, var(--orange), #ffb36b);
      color:#fff;
      font-weight:900;
      cursor:pointer;
      box-shadow:0 18px 38px rgba(221,107,32,0.20);
    }
    .fields {
      display:grid;
      gap:12px;
      grid-template-columns:repeat(auto-fit,minmax(220px,1fr));
    }
    .sectionLabel {
      display:block;
      margin-bottom:8px;
      color:var(--muted);
      font-size:13px;
      font-weight:800;
      letter-spacing:0.08em;
      text-transform:uppercase;
    }
    .summary {
      margin-top:16px;
      padding:18px;
      border-radius:22px;
      background:linear-gradient(135deg, #0f766e, #155e75);
      color:#f6fffd;
    }
    .summary .big { color:#fff; }
    .orderCard {
      border:1px solid var(--line);
      border-radius:22px;
      padding:18px;
      background:#fbfcfd;
    }
    .pill {
      display:inline-flex;
      align-items:center;
      padding:6px 10px;
      border-radius:999px;
      font-size:12px;
      font-weight:900;
      letter-spacing:0.08em;
      text-transform:uppercase;
    }
    .awaiting_payment { background:rgba(221,107,32,0.12); color:#b45309; }
    .provisioning { background:rgba(15,118,110,0.12); color:#0f766e; }
    .active { background:rgba(15,118,110,0.12); color:#0f766e; }
    .cancelled { background:rgba(185,28,28,0.10); color:#b91c1c; }
    @media (max-width: 920px) {
      .hero { grid-template-columns:1fr; }
    }
  </style>
</head>
<body>
  <div class="shell">
    <section class="hero">
      <div class="panel">
        <div class="muted">Кабинет клиента</div>
        <h1>{{.BrandName}} · ваши ключи и продление</h1>
        <p class="muted" style="margin:0; line-height:1.7;">Здесь собраны все оплаченные заказы, активные ключи устройств и быстрый конструктор для продления или покупки новых устройств.</p>
        <div class="siteKey" style="margin-top:18px;">
          <strong>Ключ кабинета</strong>
          <pre>{{.AccountToken}}</pre>
          <div class="muted" style="margin-top:10px;">Прямая ссылка: <a href="{{.AccountURL}}">{{.AccountURL}}</a></div>
        </div>
        <div class="kpiGrid" style="margin-top:18px;">
          <div class="card">
            <div class="muted">Заказов</div>
            <div class="big">{{len .Orders}}</div>
          </div>
          <div class="card">
            <div class="muted">Активных ключей</div>
            <div class="big">{{len .ActiveKeys}}</div>
          </div>
          <div class="card">
            <div class="muted">Контакт</div>
            <div class="big" style="font-size:22px;">{{.Contact}}</div>
          </div>
        </div>
      </div>

      <div class="panel">
        <div class="builder">
          <h2 style="margin:0 0 12px; font-size:24px; letter-spacing:-0.03em;">Продлить или добавить устройства</h2>
          <form method="post" action="/order">
            <input type="hidden" name="account_id" value="{{.AccountID}}">
            <input type="hidden" name="account_token" value="{{.AccountToken}}">
            <input type="hidden" name="device_count" id="renew_device_count" value="{{.DefaultQuote.DeviceCount}}">
            <input type="hidden" name="months" id="renew_months" value="{{.DefaultQuote.Months}}">

            <label class="sectionLabel" for="renew_range">Количество устройств</label>
            <input id="renew_range" type="range" min="{{.Pricing.MinDevices}}" max="{{.Pricing.MaxDevices}}" value="{{.DefaultQuote.DeviceCount}}">
            <div class="muted">Сейчас выбрано: <strong id="renew_device_label">{{.DefaultQuote.DeviceCount}}</strong></div>

            <div>
              <span class="sectionLabel">Срок</span>
              <div class="months" id="renew_months_grid">
                {{range .Pricing.MonthOptions}}
                <button class="monthBtn" type="button" data-months="{{.Months}}" data-discount="{{.DiscountPercent}}">
                  <span>{{.Label}}</span>
                  <span class="sub">{{if gt .DiscountPercent 0}}Скидка {{.DiscountPercent}}%{{else}}Без скидки{{end}}</span>
                </button>
                {{end}}
              </div>
            </div>

            <div class="summary">
              <div class="muted" style="color:rgba(246,255,253,0.8);">Новый заказ</div>
              <div class="big" id="renew_total_price">{{formatMoney .DefaultQuote.TotalMinor .DefaultQuote.Currency}}</div>
              <div id="renew_summary_line">{{.DefaultQuote.DeviceCount}} устройств · {{.DefaultQuote.Months}} мес. · скидка {{.DefaultQuote.DiscountPercent}}%</div>
            </div>

            <div class="fields" style="margin-top:14px;">
              <div>
                <label class="sectionLabel" for="renew_customer_name">Имя</label>
                <input id="renew_customer_name" name="customer_name" value="{{.CustomerName}}" required>
              </div>
              <div>
                <label class="sectionLabel" for="renew_contact">Контакт</label>
                <input id="renew_contact" name="contact" value="{{.Contact}}" required>
              </div>
              <div>
                <label class="sectionLabel" for="renew_device_label_input">Подпись для слотов</label>
                <input id="renew_device_label_input" name="device_label" placeholder="Например, Family / Office">
              </div>
              <div>
                <label class="sectionLabel" for="renew_promo_code">Промокод</label>
                <input id="renew_promo_code" name="promo_code" value="{{.PromoCode}}" placeholder="Скидка или тестовый код">
              </div>
            </div>
            <div style="margin-top:14px;">
              <label class="sectionLabel" for="renew_note">Комментарий</label>
              <textarea id="renew_note" name="note" placeholder="Например, продление текущего доступа или новые устройства"></textarea>
            </div>
            <button type="submit" style="margin-top:14px;">Создать заказ на продление</button>
          </form>
        </div>
      </div>
    </section>

    <section class="panel">
      <h2 style="margin:0 0 14px; font-size:24px; letter-spacing:-0.03em;">Активные ключи</h2>
      <div class="keyGrid">
        {{range .ActiveKeys}}
        <div class="card">
          <strong>{{.Label}}</strong>
          <div class="muted" style="margin-top:8px;">Заказ: <a href="{{.OrderURL}}">{{.OrderID}}</a></div>
          <div class="muted" style="margin-top:6px;">ID клиента: {{.ClientID}}</div>
          {{if .Subscription}}<pre>{{.Subscription}}</pre>{{end}}
          {{if .VLESS}}<div class="muted" style="margin-top:10px;">Резервный VLESS</div><pre>{{.VLESS}}</pre>{{end}}
        </div>
        {{else}}
        <div class="card">Активных ключей пока нет.</div>
        {{end}}
      </div>
    </section>

    <section class="panel">
      <h2 style="margin:0 0 14px; font-size:24px; letter-spacing:-0.03em;">История заказов</h2>
      <div class="orderGrid">
        {{range .Orders}}
        <div class="orderCard">
          <span class="pill {{statusClass .Status}}">{{statusLabel .Status}}</span>
          <h3 style="margin:12px 0 8px;">{{.PlanName}}</h3>
          <div class="muted">ID: <a href="/order/{{.ID}}?token={{.AccessToken}}">{{.ID}}</a></div>
          <div class="muted" style="margin-top:6px;">{{.DeviceCount}} устройств · {{.Months}} мес. · {{formatMoney .PriceMinor .Currency}}</div>
          <div class="muted" style="margin-top:6px;">Создан: {{formatTime .CreatedAt}}</div>
          {{if .ActivatedAt}}<div class="muted" style="margin-top:6px;">Активирован: {{formatTimePtr .ActivatedAt}}</div>{{end}}
          {{if .ProvisionError}}<div class="muted" style="margin-top:10px; color:#b91c1c;">Ошибка: {{.ProvisionError}}</div>{{end}}
        </div>
        {{end}}
      </div>
    </section>

    <section class="panel">
      <div class="grid">
        <div class="card">
          <strong>Happ</strong>
          <div class="muted" style="margin-top:8px;">{{if .HappDownloadURL}}<a href="{{.HappDownloadURL}}" target="_blank" rel="noreferrer">Открыть / скачать</a>{{else}}Ссылка не задана{{end}}</div>
        </div>
        <div class="card">
          <strong>Android APK</strong>
          <div class="muted" style="margin-top:8px;">{{if .AndroidLauncherURL}}<a href="{{.AndroidLauncherURL}}" target="_blank" rel="noreferrer">Скачать</a>{{else}}Ссылка не задана{{end}}</div>
        </div>
        <div class="card">
          <strong>Поддержка</strong>
          <div class="muted" style="margin-top:8px;">{{if .SupportLink}}<a href="{{.SupportLink}}" target="_blank" rel="noreferrer">{{.SupportLink}}</a>{{else}}Добавьте ` + "`support_link`" + ` в конфиге pay-service{{end}}</div>
        </div>
      </div>
    </section>
  </div>

  <script>
    (() => {
      const pricing = {
        baseMonthly: {{.Pricing.BaseMonthlyPriceMinor}},
        currency: {{printf "%q" .Pricing.Currency}},
        options: {{toJSON .Pricing.MonthOptions}}
      };
      const currency = pricing.currency || "RUB";
      const range = document.getElementById("renew_range");
      const devicesInput = document.getElementById("renew_device_count");
      const monthsInput = document.getElementById("renew_months");
      const devicesLabel = document.getElementById("renew_device_label");
      const totalPrice = document.getElementById("renew_total_price");
      const summaryLine = document.getElementById("renew_summary_line");
      const buttons = Array.from(document.querySelectorAll("#renew_months_grid .monthBtn"));

      function money(value) {
        if ((currency || "").toUpperCase() === "RUB") return value + " ₽";
        return value + " " + currency;
      }

      function selectedOption() {
        const months = Number(monthsInput.value || 0);
        return pricing.options.find(item => item.months === months) || pricing.options[0];
      }

      function render() {
        const devices = Number(range.value || 1);
        const option = selectedOption();
        const subtotal = pricing.baseMonthly * devices * option.months;
        const discount = Math.floor(subtotal * option.discount_percent / 100);
        const total = subtotal - discount;
        devicesInput.value = String(devices);
        devicesLabel.textContent = String(devices);
        totalPrice.textContent = money(total);
        summaryLine.textContent = devices + " устройств · " + option.months + " мес. · скидка " + option.discount_percent + "%";
        buttons.forEach(btn => {
          btn.classList.toggle("active", Number(btn.dataset.months) === option.months);
        });
      }

      buttons.forEach(btn => {
        btn.addEventListener("click", () => {
          monthsInput.value = btn.dataset.months;
          render();
        });
      });
      range.addEventListener("input", render);
      render();
    })();
  </script>
</body>
</html>`

const moderatorLoginTemplate = `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Вход модератора</title>
  <style>
    body {
      margin:0;
      min-height:100vh;
      display:grid;
      place-items:center;
      background:linear-gradient(180deg, #0b1118, #121c27);
      color:#f4f7fb;
      font-family:"Aptos","Segoe UI Variable","Trebuchet MS",sans-serif;
    }
    form {
      width:min(420px, calc(100vw - 28px));
      padding:24px;
      border-radius:26px;
      border:1px solid rgba(255,255,255,0.08);
      background:#141d28;
      box-shadow:0 20px 50px rgba(0,0,0,0.30);
    }
    input, button {
      width:100%;
      border-radius:16px;
      border:1px solid rgba(255,255,255,0.10);
      padding:13px 14px;
      font:inherit;
      margin-top:12px;
      background:#0e1620;
      color:#f4f7fb;
    }
    button {
      border:none;
      background:linear-gradient(135deg, #0f766e, #38b2ac);
      font-weight:900;
      cursor:pointer;
    }
    .notice { min-height:20px; color:#ffbcbc; }
  </style>
</head>
<body>
  <form method="post" action="/moderator/login">
    <h1 style="margin:0 0 10px;">Вход модератора</h1>
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
  <title>{{.BrandName}} · заказы и промокоды</title>
  <style>
    :root {
      --bg:#0d131c;
      --panel:#121b26;
      --line:rgba(255,255,255,0.08);
      --ink:#eef5ff;
      --muted:#93a0ae;
    }
    * { box-sizing:border-box; }
    body {
      margin:0;
      background:var(--bg);
      color:var(--ink);
      font-family:"Aptos","Segoe UI Variable","Trebuchet MS",sans-serif;
    }
    a { color:#98ebe0; text-decoration:none; }
    .shell { max-width:1260px; margin:0 auto; padding:24px 18px 48px; }
    .panel {
      background:var(--panel);
      border:1px solid var(--line);
      border-radius:24px;
      padding:18px;
    }
    table { width:100%; border-collapse:collapse; margin-top:16px; }
    th, td {
      text-align:left;
      padding:12px 10px;
      border-bottom:1px solid var(--line);
      vertical-align:top;
    }
    th {
      color:var(--muted);
      font-size:12px;
      letter-spacing:0.08em;
      text-transform:uppercase;
    }
    .muted { color:var(--muted); }
    .pill {
      display:inline-flex;
      padding:5px 10px;
      border-radius:999px;
      font-size:12px;
      font-weight:900;
      letter-spacing:0.08em;
      text-transform:uppercase;
      background:rgba(255,255,255,0.08);
    }
    .awaiting_payment { color:#ffcf9d; }
    .provisioning { color:#a9efe7; }
    .active { color:#9ce7de; }
    .cancelled { color:#ffbcbc; }
    .grid {
      display:grid;
      gap:16px;
      grid-template-columns:repeat(auto-fit, minmax(280px, 1fr));
    }
    .formGrid {
      display:grid;
      gap:12px;
      grid-template-columns:repeat(auto-fit, minmax(180px, 1fr));
      margin-top:16px;
    }
    input, button {
      width:100%;
      border-radius:14px;
      border:1px solid rgba(255,255,255,0.10);
      padding:12px 13px;
      font:inherit;
      background:#0f1722;
      color:var(--ink);
    }
    button {
      border:none;
      cursor:pointer;
      background:linear-gradient(135deg, #0f766e, #38b2ac);
      color:#042322;
      font-weight:900;
    }
    .promoCard {
      border:1px solid var(--line);
      border-radius:18px;
      padding:14px;
      background:rgba(255,255,255,0.02);
    }
    .promoMeta {
      display:grid;
      gap:8px;
      margin-top:12px;
      color:var(--muted);
      font-size:14px;
    }
    .token {
      font-family:"Cascadia Code","Consolas",monospace;
      font-size:13px;
      word-break:break-all;
    }
  </style>
</head>
<body>
  <div class="shell">
    <div class="panel">
      <div style="display:flex; justify-content:space-between; gap:14px; align-items:center; flex-wrap:wrap;">
        <div>
          <h1 style="margin:0;">{{.BrandName}} · заказы и промокоды</h1>
          <div class="muted">Управление скидочными и тестовыми промокодами, плюс просмотр всех заказов и кабинетов.</div>
        </div>
        <a href="/moderator/logout">Выйти</a>
      </div>
      <div style="margin-top:18px;">
        <h2 style="margin:0;">Создать промокод</h2>
        <div class="muted" style="margin-top:6px;">Промокод со скидкой ` + "`100%`" + ` активирует заказ без СБП и сразу выпускает ключи.</div>
        <form method="post" action="/moderator/promos">
          <div class="formGrid">
            <input name="code" placeholder="Код, например test100" required>
            <input name="name" placeholder="Название, например Тест без оплаты" required>
            <input name="discount_percent" type="number" min="0" max="100" placeholder="Скидка, %" required>
            <input name="max_uses" type="number" min="0" placeholder="Лимит использований, 0 = без лимита">
            <input name="expires_in_hours" type="number" min="0" placeholder="Срок жизни в часах, 0 = без срока">
            <button type="submit">Создать промокод</button>
          </div>
        </form>
      </div>
      <div class="grid" style="margin-top:16px;">
        {{range .Promos}}
        <div class="promoCard">
          <div style="display:flex; justify-content:space-between; gap:12px; align-items:flex-start;">
            <div>
              <strong>{{.Name}}</strong>
              <div class="token" style="margin-top:8px;">{{.Code}}</div>
            </div>
            <span class="pill {{if .Active}}active{{else}}cancelled{{end}}">{{if .Active}}активен{{else}}неактивен{{end}}</span>
          </div>
          <div class="promoMeta">
            <div>Скидка: <strong style="color:#eef5ff;">{{.DiscountPercent}}%</strong></div>
            <div>Использовано: <strong style="color:#eef5ff;">{{.UsedCount}}</strong>{{if gt .MaxUses 0}} / {{.MaxUses}}{{else}} / без лимита{{end}}</div>
            <div>Создан: <strong style="color:#eef5ff;">{{formatTime .CreatedAt}}</strong></div>
            <div>Истекает: <strong style="color:#eef5ff;">{{if .ExpiresAt}}{{formatTimePtr .ExpiresAt}}{{else}}без срока{{end}}</strong></div>
          </div>
        </div>
        {{else}}
        <div class="promoCard">Промокоды пока не созданы.</div>
        {{end}}
      </div>
      <table>
        <thead>
          <tr>
            <th>Заказ</th>
            <th>Кабинет</th>
            <th>Клиент</th>
            <th>Тариф</th>
            <th>Статус</th>
            <th>Итого</th>
          </tr>
        </thead>
        <tbody>
          {{range .Orders}}
          <tr>
            <td><a href="/moderator/orders/{{.ID}}">{{.ID}}</a><div class="muted">{{formatTime .CreatedAt}}</div></td>
            <td>{{.AccountID}}<div class="muted">{{.AccountToken}}</div></td>
            <td>{{.CustomerName}}<div class="muted">{{.Contact}}</div></td>
            <td>{{.DeviceCount}} устройств · {{.Months}} мес.<div class="muted">{{.PlanName}}</div>{{if .PromoCode}}<div class="muted">Промокод: {{.PromoCode}}{{if .PromoDiscountPercent}} · скидка {{.PromoDiscountPercent}}%{{end}}</div>{{end}}</td>
            <td><span class="pill {{.Status}}">{{statusLabel .Status}}</span>{{if .ProvisionError}}<div class="muted" style="margin-top:6px;">{{.ProvisionError}}</div>{{end}}</td>
            <td>{{formatMoney .PriceMinor .Currency}}</td>
          </tr>
          {{else}}
          <tr><td colspan="6">Заказы пока не найдены.</td></tr>
          {{end}}
        </tbody>
      </table>
    </div>
  </div>
</body>
</html>`
