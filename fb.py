import requests
import random
import string

# iOS User-Agent (Facebook App)
USER_AGENT = (
    "Mozilla/5.0 (iPhone; CPU iPhone OS 18_3_1 like Mac OS X) AppleWebKit/605.1.15 "
    "(KHTML, like Gecko) Mobile/22D72 [FBAN/FBIOS;FBAV/503.0.0.56.104;FBDV/iPhone12,8;FBMD/iPhone;"
    "FBSN/iOS;FBSV/18.3.1;FBSS/2;FBID/phone;FBLC/nl_NL;FBOP/5;FBRV/708854960;IABMV/1]"
)

# Facebook App Token
token_app = "6628568379|c1e620fa708a1d5696fb991c1bde5662"

# Generate random US phone number
def generate_random_us_phone():
    area_codes = ["201", "202", "203", "205", "206", "207", "208", "209", "210", "212", "213", "214", "215", "216", "217", "218", "219", "220", "224"]
    area_code = random.choice(area_codes)
    middle = random.randint(100, 999)  # 100-999
    last = random.randint(1000, 9999)  # 1000-9999
    return f"+1{area_code}{middle}{last}"  # Format: +12025550123

# Fetch random user details from API
def fetch_random_user():
    try:
        response = requests.get("https://randomuser.me/api/", timeout=5)
        user = response.json()['results'][0]

        firstname = user['name']['first']
        lastname = user['name']['last']
        gender = "M" if user['gender'] == "male" else "F"
        dob = user['dob']['date'].split("T")[0]  # Format: YYYY-MM-DD

        return {'firstname': firstname, 'lastname': lastname, 'gender': gender, 'dob': dob}
    except Exception as e:
        print("Random user fetch failed:", e)
        return None

# Register Facebook Account
def register_facebook():
    print("Fetching random user...")
    user = fetch_random_user()

    if not user:
        print("Failed to generate random user, exiting...")
        return

    phone = generate_random_us_phone()
    print(f"Generated User: {user['firstname']} {user['lastname']} | {phone} | {user['gender']} | {user['dob']}")

    params = {
        'pretty': "0",
        'generate_session_cookies': "1",
        'generate_machine_id': "1",
        'return_ssl_resources': "0",
        'return_multiple_errors': "true",
        'attempt_login': "1",
        'format': "json",
        'credentials_type': "password",
        'password': "haidanh912",
        'gender': user['gender'],
        'birthday': user['dob'],
        'firstname': user['firstname'],
        'lastname': user['lastname'],
        'phone': phone,
        'v': "1.0",
        'access_token': token_app,
    }

    try:
        response = requests.get(
            "https://graph.facebook.com/restserver.php?method=user.register",
            params=params,
            headers={"User-Agent": USER_AGENT},
            timeout=100
        )

        print("Response:", response.json())
    except Exception as e:
        print("Request failed:", e)

# Run script
register_facebook()
