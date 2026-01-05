-- Create databases for each service
CREATE DATABASE orchid;
CREATE DATABASE lotus;
CREATE DATABASE ivy;

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE orchid TO "user";
GRANT ALL PRIVILEGES ON DATABASE lotus TO "user";
GRANT ALL PRIVILEGES ON DATABASE ivy TO "user";
