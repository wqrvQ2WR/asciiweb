# AsciiWeb

웹이 보이는 그대로를 글자로 그리는 브라우저.
기본 동작은 일반 브라우저이고(클릭, 스크롤, 입력), 화면만 문자로 렌더링한다. 신개념 w3m.

두 가지 모드 (진짜 렌더링은 Chrome/WebView 엔진이라 HTML5와 CSS3를 그대로 지원한다. 텍스트 모드는 인라인 스타일 정도만):

- **진짜 렌더링 (기본):** 실제 브라우저 엔진이 그린 화면을 글자로 변환한다.
  - 맥/윈도우/리눅스: 설치된 Chrome, Edge, Brave를 CDP로 제어 (Go 단일 실행 파일)
  - 안드로이드: 시스템 WebView 화면을 캡처해서 변환, 터치는 그대로 페이지에 전달
- **텍스트 재배치 (`-text`, 가볍다):** HTML을 직접 파싱해서 문자 격자로 다시 배치하고, 이미지는 ASCII 아트로 바꾼다. 브라우저가 필요 없고 메모리 약 20~30MB.

## 설정 (화질과 표현 방식)

데스크톱은 `Ctrl-S`, 안드로이드는 메뉴의 Settings.

| 항목 | 값 |
|---|---|
| Style (표현 방식) | ascii, detail(70단계), blocks(반블록 진짜 픽셀), braille(점자 2x4) |
| Color | auto, none, 256, true (안드로이드는 on/off) |
| Real text overlay | 페이지의 글자를 진짜 문자로 찍어서 또렷하게 (기본 켜짐). 이미지와 배경만 ASCII 아트 |
| Bold characters | 글자를 굵게 (기본 켜짐) |
| Invert | auto, on, off (밝은 페이지의 어두운 잉크를 진하게) |
| Contrast | -4 ~ +4 |
| Page width (화질) | 페이지 레이아웃 폭(CSS px). auto는 칸 수에 맞춰 자동 (한 글자가 1~2칸). 좁을수록 글자가 커진다 |
| Frame quality / Sampling | JPEG 품질(데스크톱), 샘플링 배율(안드로이드) |
| Frame rate | 초당 프레임 상한 |
| Mobile sites | 모바일 UA |
| Search engine | duckduckgo, google, naver, bing |
| Home page | 현재 페이지로 지정 |
| Text size | (안드로이드) 작을수록 칸이 많아 세밀 |

설정은 데스크톱에서 `settings.json`(OS 설정 폴더/asciiweb)에 저장된다.
글자를 알아보려면 터미널 칸 수가 중요하다. 터미널 글꼴을 줄이거나 창을 키우고, Page width를 줄이고, braille을 써 보면 좋아진다.

## 데스크톱 사용법

```sh
./build.sh                          # dist/ 에 6개 타깃 빌드
./dist/asciiweb-darwin-arm64        # 홈 페이지
./dist/asciiweb-darwin-arm64 news.ycombinator.com
./dist/asciiweb-darwin-arm64 -text  # 가벼운 텍스트 재배치 모드
./dist/asciiweb-darwin-arm64 -shot -style braille -width 480 -cols 120 https://example.com  # 한 프레임만 출력
```

| 입력 | 동작 |
|---|---|
| 마우스 클릭 / 휠 | 페이지 클릭 / 스크롤 |
| 글자 키, Enter, Tab, 방향키, PgUp/PgDn | 그대로 페이지에 전달 (입력칸에 타이핑, 한글 가능) |
| Ctrl-L | 주소 또는 검색어 입력 (맨 윗줄 클릭도 같음) |
| Ctrl-B / Ctrl-F / Ctrl-R / Ctrl-H | 뒤로 / 앞으로 / 새로고침 / 홈 |
| Ctrl-S | 설정 |
| Ctrl-Q | 종료 |

Chrome 계열 브라우저가 없으면 자동으로 텍스트 모드로 시작한다. 경로는 `ASCIIWEB_CHROME`로 지정할 수 있다.
구글은 자동 제어되는 브라우저로 보면 확인 페이지(/sorry/)를 띄울 수 있다. 기본 검색엔진은 DuckDuckGo.
진짜 렌더링 모드는 Chrome 프로세스를 함께 쓰므로 램이 더 든다(텍스트 모드가 가볍다).

## 안드로이드

`android/` 를 Android Studio로 열거나 `cd android && ./gradlew assembleDebug`. 릴리스에 APK가 있다.
기본은 진짜 렌더링. 화면을 탭하면 그대로 페이지에 전달되고 키보드로 입력한다. 설정에서 텍스트 재배치 모드로 바꿀 수 있다.

## 메모리

위키백과 문서(이미지 수십 장 포함)를 열었을 때 최대 약 31MB, 이미지 없는 Hacker News는 약 21MB (맥 arm64, `/usr/bin/time -l`).
이미지는 받자마자 최대 216x400 RGB로 줄이고 원본은 버린다.

## 텍스트 모드 (`-text`)

```sh
./build.sh                       # dist/ 에 6개 타깃 빌드
./dist/asciiweb-darwin-arm64     # 시작 화면
./dist/asciiweb-darwin-arm64 news.ycombinator.com
./dist/asciiweb-darwin-arm64 ascii art    # 주소가 아니면 검색(DuckDuckGo)
./dist/asciiweb-darwin-arm64 -dump -plain -cols 100 https://example.com   # 한 번 그리고 종료
```

| 키 | 동작 |
|---|---|
| j k / 방향키 / 마우스 휠 | 스크롤 |
| Space b / PgDn PgUp | 한 페이지 |
| g G | 맨 위 / 맨 아래 |
| Tab, Shift-Tab | 다음 / 이전 링크·입력칸 |
| Enter, 마우스 클릭 | 링크 열기, 입력칸 편집 후 전송, 체크박스 토글, 선택지 바꾸기, 버튼 누르기 |
| o / O | 주소 또는 검색어 열기 / 현재 주소 고쳐서 열기 |
| H L | 뒤로 / 앞으로 |
| r | 새로고침 |
| / n N | 찾기, 다음, 이전 |
| i | 이미지 켜기/끄기 |
| c | 색 켜기/끄기 |
| h, ? | 홈, 도움말 |
| q | 종료 |

플래그: `-no-images`, `-no-color`, `-dump`, `-plain`, `-cols N`.
윈도우는 Windows Terminal(또는 Win10 이후 콘솔)에서 동작하도록 VT 모드를 켜게 만들었다. 윈도우 실행은 이 저장소 작성 환경에서 직접 돌려 보지 못했다(컴파일과 vet만 확인).

## 안드로이드

`android/` 를 Android Studio로 열거나 `cd android && ./gradlew assembleDebug`. 릴리스에 APK가 있다.
링크 탭, 입력칸 탭(대화상자), 뒤로가기, 주소창 검색, 이미지/색/글자 크기 메뉴, 외부 링크 열기(http/https 인텐트).

## 구조

```
engine/            Go 코어(텍스트 모드): HTML 파싱, 줄바꿈, 표, 목록, 폼, 이미지 ASCII 변환
render/            진짜 렌더링: Chrome(CDP) 세션, 화면->글자 변환기, 설정
cmd/asciiweb/      터미널 UI (키 입력, 마우스, 그라디언트 상태바)
android/           Kotlin 포팅 + Compose UI (app/src/main/java/.../engine 가 코어)
```

렌더링 규칙: 블록/인라인 흐름, 한글 2칸 폭, h1-h6, 목록, 인용, pre, 표(경계선과 열 너비 배분),
폼(텍스트, 체크박스, 라디오, select, 버튼, GET/POST), 인라인 style의 color / display:none / text-align.
이미지는 가장자리가 밝으면 종이로 보고 어두운 잉크 쪽을 진하게 그린다(로고, 도표용).

## 한계

- JavaScript와 스타일시트는 실행하지 않는다 (w3m과 같다). 앱형 사이트는 HTML 기본 모습만 나온다.
- 이미지는 JPEG, PNG, GIF, WebP, BMP. SVG는 대체 텍스트로 나온다.
- 다운로드, 탭, 북마크 없음.

## 테스트

```sh
go test ./...                              # 엔진
cd android && ./gradlew testDebugUnitTest  # Kotlin 엔진 (같은 시나리오)
```
