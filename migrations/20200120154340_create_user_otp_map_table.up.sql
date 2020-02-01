CREATE TABLE IF NOT EXISTS UserOtp (
    userId int(11) UNSIGNED NOT NULL,
    otpId int(11) UNSIGNED NOT NULL,
    FOREIGN KEY (userId) REFERENCES Users(id),
    FOREIGN KEY (otpId) REFERENCES OTP(id)
);