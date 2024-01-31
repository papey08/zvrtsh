CREATE TABLE "users" (
    "name" VARCHAR(30) PRIMARY KEY,
    "encoded_password" VARCHAR(250),
    "secret_code" VARCHAR(36)
);
