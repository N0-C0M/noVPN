# Desktop Scaffold

Минимальный desktop-каркас лежит в этой директории.

Что уже есть:

- загрузка профиля Reality из `client/common/profiles/reality/default.profile.json`;
- генерация локального `config.json` для Xray;
- чекбокс `Не проксировать РФ`;
- выбор desktop `.exe` исключений;
- headless CLI и Tkinter UI.

Быстрый старт:

```bash
python client/desktop/python/app.py --headless --bypass-ru --exclude-app "C:\Program Files\Telegram Desktop\Telegram.exe"
```
