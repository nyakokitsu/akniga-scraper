# akniga-scraper
> A small akniga.org downloader written in golang.
> Мелкая качалка с akniga.org написанная на golang.

# Download/Скачать
Позже.
## Зависимости
_**Для скачивания аудио нужен ffmpeg!**_ Убедитесь что он у вас стоит и есть в PATH.

# How it works?/Как оно работать?
Получаются нужные куки, получается bookid(bid), затем идет `POST https://akniga.org/ajax/b/{bid}` из которого получается hres, который зашифрован. Декод в `cryptoutil/decrypter.go` с изпользованием ключа вытащенного из js-ки сайта и получается hls на аудио. Бинго.

# Прочее
кофе помогает от Альцгеймера
