# cwt

`cwt`는 Codex CLI용 Go 기반 worktree 실행기입니다.

기본 아이디어는 단순합니다.

1. Codex를 열기 전에 Git worktree를 먼저 고릅니다.
2. picker 흐름 안에서 worktree를 생성하거나 삭제할 수 있습니다.
3. 마지막 선택 시각, Codex 세션 수, 마지막 Codex 활동 시각 같은 메타데이터를 SQLite에 저장합니다.

WSL에서 바로 쓰기 쉬운 Linux 단일 바이너리 배포를 첫 목표로 둡니다.

정식 명세는 [docs/spec.md](docs/spec.md)를 참고하면 됩니다.

## 주요 기능

- `cwt` 실행 시 worktree picker를 띄운 뒤 선택한 worktree에서 `codex`를 실행합니다.
- `cwt resume ...`도 동일하게 picker를 띄운 뒤 `codex resume ...`로 위임합니다.
- `cwt list`로 branch, 세션 수, 오래됨 힌트가 포함된 목록을 봅니다.
- `cwt create`로 새 sibling worktree를 생성합니다.
- `cwt remove`로 worktree를 제거합니다.
- `cwt cleanup`으로 오래된 worktree 후보를 찾거나 정리합니다.
- 상태 DB는 `$XDG_STATE_HOME/cwt/index.sqlite` 또는 `~/.local/state/cwt/index.sqlite`에 저장됩니다.
- Codex 세션 정보는 `$CODEX_HOME/sessions` 또는 `~/.codex/sessions`를 스캔해서 추론합니다.

## 요구 사항

- Git
- `PATH`에 잡힌 Codex CLI
- 로컬 개발용 Go 1.24.0
- 개발 환경 정합성을 위한 `goenv` 권장

## 개발 환경 설정

```bash
goenv install 1.24.0
goenv local 1.24.0
go mod tidy
go test ./...
go build ./cmd/cwt
```

## 사용 예시

선택한 worktree에서 Codex 시작:

```bash
cwt
```

선택한 worktree에서 resume:

```bash
cwt resume --last
```

직접 worktree 생성:

```bash
cwt create feature-login
cwt create --branch feat/login-timeout feature-login
```

worktree와 메타데이터 목록 보기:

```bash
cwt list
```

오래된 worktree 후보만 확인:

```bash
cwt cleanup --stale-days 30
```

오래된 worktree 실제 정리:

```bash
cwt cleanup --stale-days 30 --apply
```

worktree 제거:

```bash
cwt remove ../repo--feature-login
```

## Picker 조작

- `<번호>`: worktree 선택
- `c`: worktree 생성
- `d<number>`: 표시된 worktree 삭제
- `r`: 새로고침
- `q`: 종료

## 데이터 모델

SQLite 테이블:

- `worktrees`
- `sessions`
- `events`

대표적으로 저장하는 항목:

- path
- branch
- main / locked / prunable 상태
- 마지막 선택 시각
- 마지막 Codex 활동 시각
- Codex 세션 수
- launch 횟수

## 테스트

```bash
go test ./...
```

현재 자동화된 테스트 범위:

- `git worktree --porcelain` 파싱
- Codex 세션 로그 스캔
- 오래된 worktree 판정
- 명령 위임 계획
- SQLite 저장소 동작
- picker 입력 파싱

## CI 및 릴리스

- CI는 `go mod tidy`, `go test ./...`, `go build ./cmd/cwt`를 실행합니다.
- `v*` 태그를 푸시하면 GitHub Release 워크플로가 동작합니다.
- 릴리스 산출물:
  - `cwt-linux-amd64`
  - `cwt-linux-amd64.tar.gz`
  - `sha256sum.txt`

## 참고

- `cwt --help`, `cwt --version`은 picker 없이 `codex`에 바로 위임합니다.
- `v0.0.1`에서는 풀스크린 TUI 대신, 신뢰성 우선의 줄 기반 터미널 picker를 사용합니다.
