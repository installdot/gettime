from flask import Flask, Response
import requests
import re
from bs4 import BeautifulSoup
from datetime import datetime, timedelta

app = Flask(__name__)

def get_release_time():
    url = "https://genshin-countdown.gengamer.in"
    
    # Fetch the webpage content
    response = requests.get(url)
    if response.status_code != 200:
        return "Không thể lấy dữ liệu từ trang web."

    # Parse the HTML using BeautifulSoup
    soup = BeautifulSoup(response.text, "html.parser")

    # Find the script tag containing "const releaseTimeAS"
    script_tags = soup.find_all("script")
    release_time_str = None
    
    for script in script_tags:
        if script.string and "const releaseTimeAS" in script.string:
            match = re.search(r"new Date\('(.+?)'\)", script.string)
            if match:
                release_time_str = match.group(1)
                break

    if not release_time_str:
        return "Không tìm thấy thời gian phát hành."

    # Remove ' UTC+8' and parse the date
    release_time_str = release_time_str.replace(" UTC+8", "")
    release_time = datetime.strptime(release_time_str, "%B %d, %Y %H:%M:%S")

    # Convert release time from UTC+8 to GMT+7
    release_time = release_time - timedelta(hours=1)

    # Get current time in GMT+7
    current_time = datetime.utcnow() + timedelta(hours=7)

    # Calculate the time difference
    time_difference = release_time - current_time

    if time_difference.total_seconds() < 0:
        return "Thời gian phát hành đã qua."

    # Convert to days, hours, minutes, seconds
    days = time_difference.days
    hours, remainder = divmod(time_difference.seconds, 3600)
    minutes, seconds = divmod(remainder, 60)

    # Format the release date in Vietnamese
    weekdays = {
        "Monday": "Thứ Hai",
        "Tuesday": "Thứ Ba",
        "Wednesday": "Thứ Tư",
        "Thursday": "Thứ Năm",
        "Friday": "Thứ Sáu",
        "Saturday": "Thứ Bảy",
        "Sunday": "Chủ Nhật",
    }
    months = {
        "January": "Tháng 1", "February": "Tháng 2", "March": "Tháng 3",
        "April": "Tháng 4", "May": "Tháng 5", "June": "Tháng 6",
        "July": "Tháng 7", "August": "Tháng 8", "September": "Tháng 9",
        "October": "Tháng 10", "November": "Tháng 11", "December": "Tháng 12",
    }

    formatted_weekday = weekdays[release_time.strftime("%A")]
    formatted_month = months[release_time.strftime("%B")]
    formatted_release_time = release_time.strftime(f"%I:%M %p {formatted_weekday}, %d {formatted_month} %Y")

    # Create the response text
    response_text = f"Thời gian phát hành (GMT+7): {formatted_release_time}\n"
    response_text += f"Còn lại: {days} ngày, {hours} giờ, {minutes} phút, {seconds} giây"

    return response_text

@app.route("/")
def home():
    release_info = get_release_time()
    return Response(release_info, mimetype="text/plain; charset=utf-8")

if __name__ == "__main__":
    app.run(host="0.0.0.0")
