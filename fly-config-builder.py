from base64 import b64encode
app_name = input("Enter a name for your fly.io app: ")
basic_auth_username = input("Enter the username you'd like to use to log into your app: ")
basic_auth_password = input("And the password: ")
builder_github_org_name = input("Enter the GitHub username that you used when creating the builder repo from them template\n (if you have not done so yet, do so here: https://github.com/SignTools/SignTools-CI): ")
builder_github_repo_name = input("Enter the name of the builder repo you created from the template: ")
builder_github_token = input("Enter the GitHub token you created for the builder repo (see GitHub Actions section here: https://github.com/SignTools/SignTools-CI#github-actions): ")
builder_github_workflow_file_name = input("Your builder repository's workflow file name. Leave blank if you didn't change it from the default 'sign.yml': ")
if builder_github_workflow_file_name == "":
    builder_github_workflow_file_name = "sign.yml"
profile_account_name = input("Your Apple developer account's name (e-mail): ")
profile_account_pass = input("Your Apple developer account's password: ")
profile_cert_base64 = input("Your signing profile's certificate (p12). This is always required, no matter what signing method you use. Drag and drop the file into the terminal window, or enter the path to the file: ")
if profile_cert_base64.startswith("\""):
    profile_cert_base64 = profile_cert_base64[1:]
if profile_cert_base64.endswith("\""):
    profile_cert_base64 = profile_cert_base64[:-1]
profile_cert_base64 = b64encode(open(profile_cert_base64, "rb").read()).decode("utf-8")
profile_cert_pass = input("Your signing profile's certificate password: ")
profile_name = input("A friendly name to display your signing profile on the website: ")

fly_toml = f"""app = "{app_name}" \n\
"kill_signal" = "SIGINT" \n\
"kill_timeout" = 5 \n\
"processes" = [] \n\
\n\
[env] \n\
    BASIC_AUTH_ENABLE = "true" \n\
    BASIC_AUTH_USERNAME = "{basic_auth_username}" \n\
    BASIC_AUTH_PASSWORD = "{basic_auth_password}" \n\
    BUILDER_GITHUB_ENABLED = "true" \n\
    BUILDER_GITHUB_ORG_NAME = "{builder_github_org_name}" \n\
    BUILDER_GITHUB_REPO_NAME = "{builder_github_repo_name}" \n\
    BUILDER_GITHUB_TOKEN = "{builder_github_token}" \n\
    BUILDER_GITHUB_WORKFLOW_FILE_NAME = "{builder_github_workflow_file_name}" \n\
    PROFILE_ACCOUNT_NAME = "{profile_account_name}" \n\
    PROFILE_ACCOUNT_PASS = "{profile_account_pass}" \n\
    PROFILE_CERT_BASE64 = "{profile_cert_base64}" \n\
    PROFILE_CERT_PASS = "{profile_cert_pass}" \n\
    PROFILE_NAME = "{profile_name}" \n\
    REDIRECT_HTTPS = "true" \n\
\n\
[experimental]
  allowed_public_ports = []
  auto_rollback = true

[[services]]
  http_checks = []
  internal_port = 8080
  processes = ["app"]
  protocol = "tcp"
  script_checks = []
  [services.concurrency]
    hard_limit = 25
    soft_limit = 20
    type = "connections"

  [[services.ports]]
    force_https = true
    handlers = ["http"]
    port = 80

  [[services.ports]]
    handlers = ["tls", "http"]
    port = 443

  [[services.tcp_checks]]
    grace_period = "1s"
    interval = "15s"
    restart_limit = 0
    timeout = "2s"
"""

with open("fly.toml", "w") as f:
    f.write(fly_toml)

print("Your fly.toml file has been created!")