# Data Model: Confluence Sync Application

## Configuration
- **confluence_url**: string
- **confluence_user**: string
- **confluence_token**: string

## Project
- **name**: string
- **path**: string
- **space**: string
- **parent_page_id**: string

## Page
- **id**: string
- **title**: string
- **content**: string
- **version**: int
- **attachments**: Attachment[]

## Attachment
- **id**: string
- **name**: string
- **size**: int
- **url**: string
