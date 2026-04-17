NOURISH PT Setup and Integration Guide

Overview
-> How to set up the repository...

Repository Setup
Clone the repository to your local machine and navigate into the root directory. 
You must have Docker installed. Run the command docker-compose up --build to initialize the system. 
This command starts the backend service on port 8081 and the frontend web interface on port 8082. 

Configuration
You need to configure your environment variables before starting the application. 
Copy the provided example environment file to a new file named .env in the root folder. 
Open this file and fill in your database credentials. 
This includes the database host, port, username, password, and database name. 

Data Downloads
You will receive a set of data files separately. 
Place all provided comma separated value files and datasets into the data directory located in the project root. 
The backend requires these files to calculate baseline foot traffic, commercial rent, and demographic information when the database is unreachable.
Place `data/` in the root folder alongside this readme file.
Download here: https://drive.google.com/file/d/1plN_iCzEzyYGbQvjdDaxNP6La0YSbFdq/view?usp=sharing

Connecting to the Main NOURISH Platform
The main NOURISH platform communicates with this local engine over HTTP. 
You need to point the main platform to your local backend address. 
You can test the connection by opening your web browser and navigating to localhost:8081/api/health. 
A healthy status message indicates the application programming interface is ready to receive requests from the core NOURISH platform.

Accessing the Application Programming Interface
You can explore the full documentation by visiting localhost:8081/swagger in your browser. 
This interactive interface allows you to test endpoints and view the expected request and response structures.

Endpoints and Parameters
The evaluate location endpoint is located at /api/evaluate-location and is used to score a specific site. 
It accepts a latitude parameter named lat and a longitude parameter named lng. 
You can alternatively provide a physical address string. 
You may also pass a business category code using the naics parameter or specific terms using the keyword parameter such as bakery or coffee.

The opportunity map endpoint is located at /api/opportunity-map and returns a grid of scored locations within a designated area. 
This endpoint requires bounding box coordinates. You must provide the north latitude limit as n, the south latitude limit as s, 
the east longitude limit as e, and the west longitude limit as w.

The find best match endpoint is located at /api/find-best-match and searches the designated bounding box for the most mathematically viable business opportunities. 
It accepts the same bounding box parameters as the opportunity map. It also accepts a budget parameter to filter out opportunities that exceed a specified maximum startup cost.
