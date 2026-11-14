# AsciiWeb

웹을 전부 글자로 그리는 브라우저. 신개념 w3m.
HTML을 직접 파싱해서 문자 격자로 배치하고, **이미지는 다운스케일해서 밝기와 색을 ASCII 문자로 바꿔 그린다.**
JS 엔진도 렌더링 엔진도 없어서 가볍다.

- 맥 / 윈도우 / 리눅스: Go 단일 실행 파일 (약 7MB), 터미널에서 실행
- 안드로이드: Kotlin 앱, 같은 레이아웃 규칙을 포팅 (글자 격자를 Canvas로 직접 그림)

## 메모리

위키백과 문서(이미지 수십 장 포함)를 열었을 때 최대 약 31MB, 이미지 없는 Hacker News는 약 21MB (맥 arm64, `/usr/bin/time -l`).
이미지는 받자마자 최대 216x400 RGB로 줄이고 원본은 버린다.

## 쓰는 법 (데스크톱)

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
engine/            Go 코어: HTML 파싱, 줄바꿈, 표, 목록, 폼, 이미지 ASCII 변환
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
