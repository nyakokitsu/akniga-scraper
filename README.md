# akniga-scraper
> A small akniga.org downloader written in golang.
> Мелкая качалка с akniga.org написанная на golang.

# Download/Скачать
Позже

# How it works?/Как оно работать?
Получаются нужные куки, получается bookid(bid), затем идет `POST https://akniga.org/ajax/b/{bid}` из которого получается hres, который зашифрован. Декод в `cryptoutil/decrypter.go` с изпользованием ключа вытащенного из js-ки сайта и получается hls на аудио. Бинго.
