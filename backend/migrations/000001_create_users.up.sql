CREATE TABLE "users" (
    "name"             VARCHAR(30)  PRIMARY KEY,
    "encoded_password" VARCHAR(100) NOT NULL,
    "token"            VARCHAR(64)  NOT NULL UNIQUE
);
