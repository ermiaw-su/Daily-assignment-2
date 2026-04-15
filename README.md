***API List***
1. /login for do the login in login.html
2. /register for fo the register in register.html
3. /success is an api for mockup for the success.html that use after success do the login.

**What we used**
1. Go
2. MySQL
3. JWT
4. bcrypt
5. REST API

**Database Schema**

CREATE TABLE users (
  id INT AUTO_INCREMENT PRIMARY KEY,
  username VARCHAR(50) UNIQUE,
  password TEXT
);

CREATE TABLE events (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255),
    quota INT
);

CREATE TABLE bookings (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(255),
    event_id INT
);