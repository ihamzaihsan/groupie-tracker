# Groupie Tracker Website

Groupie Tracker is a web application that receives data from a given API and manipulates the information to create a user-friendly website for displaying details about bands and artists. This project is written in Go for the backend and focuses on data visualization, event handling, and client-server communication.

## Objectives

Groupie Tracker aims to achieve the following objectives:

1. Receive data from a provided API, which consists of four parts:
   - Artists: Information about bands and artists, including their names, images, start years, first album release dates, and members.
   - Locations: Concert locations for bands and artists.
   - Dates: Concert dates for bands and artists.
   - Relation: Links between artists, dates, and locations.

2. Build a user-friendly website to display band information through various data visualizations, such as blocks, cards, tables, lists, pages, and graphics.

3. Create and visualize events and actions, focusing on client-server communication. An event may include a client call to the server to trigger specific actions and obtain information.

## Features

Key features of the Groupie Tracker website include:

- Displaying band and artist information using various data visualizations.
- Implementing client-server communication for retrieving data.
- Handling events and actions triggered by the client or other factors.
- Ensuring the site and server run without crashing and handle errors gracefully.
- Adhering to coding best practices.

## Technologies Used

- **Go**: Backend development.
- **HTML & CSS**: Frontend development.
- **JSON**: Data format for API interaction.
- **Gitea**: Version control and project management.

## Prerequisites

To run this project, you need to have the following prerequisites:

- Go programming language
- A web browser to access the site

## Installation

Follow these steps to set up and run the Groupie Tracker website:

1. Clone this repository to your local machine.
   ```shell
   git clone https://learn.reboot01.com/git/hhanoon/groupie-tracker.git
   ```

2. Change to the project directory.
   ```shell
   cd groupie-tracker
   ```

3. Run the Go application to start the server.
   ```shell
   go run backend/main.go
   ```

4. Open your web browser and navigate to [http://localhost:8080](http://localhost:8080) to access the Groupie Tracker website.

## Usage

Once the website is running, you can use it to explore information about various bands and artists. You can trigger events and actions to retrieve specific data and visualize it in different formats.

## Author

- [Hamza Cheema](https://learn.reboot01.com/git/hcheema)
- [Hussain Hanoon](https://learn.reboot01.com/git/hhanon)
- [Zainab Nabhan](https://learn.reboot01.com/git/znabhan)

## Acknowledgments

- API source: [API](https://groupietrackers.herokuapp.com/api)
