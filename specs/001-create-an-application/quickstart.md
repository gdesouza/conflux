# Quickstart: Confluence Sync Application

1. **Configure your Confluence credentials**:
   ```
   conflux configure --url <your-confluence-url> --user <your-username> --token <your-api-token>
   ```

2. **Initialize a new project**:
   ```
   conflux init --name my-project --path ./my-docs --space MYSPACE --parent-page-id 12345
   ```

3. **Create some markdown files** in the `./my-docs` directory.

4. **Sync your project with Confluence**:
   ```
   conflux sync --project my-project
   ```

5. **Download a page from Confluence**:
   ```
   conflux download --project my-project --page-id 54321
   ```
