# Slack Webhook API Spec

## Endpoint

```
POST https://slack...
```

Channel: `#notify_package`

## Request

- **Content-Type:** `application/json`

### Body

| 필드          | 타입                | 필수 | 설명                        |
| ------------- | ------------------- | ---- | --------------------------- |
| `text`        | string              | ✓    | 채널에 표시되는 메인 텍스트 |
| `attachments` | array\<Attachment\> | —    | 첨부 블록 목록              |

### Attachment

| 필드     | 타입                     | 설명                                                   |
| -------- | ------------------------ | ------------------------------------------------------ |
| `color`  | string                   | 사이드바 색상 (hex 또는 `good` / `warning` / `danger`) |
| `title`  | string                   | 제목                                                   |
| `text`   | string                   | 본문                                                   |
| `fields` | array\<AttachmentField\> | 키-값 필드 목록                                        |

### AttachmentField

| 필드    | 타입    | 설명                             |
| ------- | ------- | -------------------------------- |
| `title` | string  | 필드 제목                        |
| `value` | string  | 필드 값                          |
| `short` | boolean | `true`이면 2열 레이아웃으로 표시 |

## Example

```json
{
  "text": "배포 알림",
  "attachments": [
    {
      "color": "#FF0000",
      "title": "배포 실패",
      "text": "에러 메시지",
      "fields": [
        { "title": "gameId", "value": "123", "short": true },
        { "title": "env", "value": "production", "short": true }
      ]
    }
  ]
}
```

## Response

| 상태 코드 | Body | 설명      |
| --------- | ---- | --------- |
| 200       | `ok` | 전송 성공 |
| 4xx / 5xx | 에러 | 전송 실패 |
