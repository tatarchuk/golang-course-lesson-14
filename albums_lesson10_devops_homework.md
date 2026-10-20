# Домашнє завдання: Тема 10, додаткова частина — DevOps Basics (VM, Docker, Floci, k3s)

## Наскрізний проєкт: Albums API (система обліку музичних альбомів, варіант `albums`)

> Умову видано англійською до презентації «DevOps Basics for GenAI Developers» (SoftServe Academy,
> курс Go with GenAI); оригінальний текст наведено в англійській половині цього файлу. «Flossy» в
> умові — це Floci, локальний емулятор AWS зі слайдів 29–30 і 38. «Демо для застосунків 1 і 2» — це
> Demo 1 (EC2, слайд 32) і Demo 2 (Lambda на Go, слайди 33–36); Demo 3 (Terraform, слайд 37) умова не
> вимагає. «Go-застосунок, який ви створили» — Albums API з цього репозиторію: `Dockerfile` і
> `docker-compose.yml` у ньому вже є з попередніх тем. Робоча машина — Mac на Apple Silicon (arm64),
> тому віртуальна машина, образи й бінарники теж arm64: box `ubuntu/jammy64` зі слайда 6 існує лише
> для amd64. Пункт «**» (Kubernetes) в умові не має балів. Звіт — розділ «Lesson 10 — додаткова
> частина: DevOps Basics» у `README.md`, докази — у `docs/devops/`, скриншоти — у
> `docs/screenshots/devops/`.

**Бали:** 70 (частина 1) + 30 (частина 2); частина 3 в умові балів не має.

**Мета роботи:** зібрати навколо Albums API DevOps-середовище з презентації — віртуальну машину з Docker, емулятором AWS Floci і перевіреним кластером k3s; виконати на ньому демо з EC2 і Lambda; запустити застосунок разом із базою даних у Docker Compose і розгорнути його в Kubernetes.

---

### ЧАСТИНА 1: Середовище і демо з презентації [70 балів]

* **Завдання 1.1:** підготуйте середовище — віртуальну машину зі встановленим Docker, запущеним Floci («Flossy») і перевіреним розгортанням k3s.
* **Завдання 1.2:** виконайте демо для застосунків 1 і 2 з презентації: Demo 1 (EC2 на Floci) і Demo 2 (Lambda на Go на Floci).

---

### ЧАСТИНА 2 (*): Docker і Docker Compose [+30 балів]

* Контейнеризуйте створений Go-застосунок (Albums API) і запустіть його через Docker Compose разом із базою даних (PostgreSQL).

---

### ЧАСТИНА 3 (**): Kubernetes

* Розгорніть застосунок у Kubernetes (кластер k3s із частини 1).

---

### Вимоги до здачі домашнього завдання (Deliverables)

1. Скриншоти або відеозапис, що демонструють роботу застосунку.
2. Посилання на GitHub-репозиторій із конфігураціями Docker і Docker Compose.

---

# Homework Assignment: Lesson 10, extra part — DevOps Basics (VM, Docker, Floci, k3s)

## Capstone Project: Albums API (music album catalogue, the `albums` variant)

> The assignment came in English with the deck "DevOps Basics for GenAI Developers" (SoftServe
> Academy, Go with GenAI); its text is quoted verbatim below. "Flossy" is Floci, the local AWS
> emulator of slides 29–30 and 38. "The demos for applications 1 and 2" are Demo 1 (EC2, slide 32)
> and Demo 2 (a Go Lambda, slides 33–36); Demo 3 (Terraform, slide 37) is not required. "The Go
> application you created" is this repository's Albums API, which already has a `Dockerfile` and a
> `docker-compose.yml` from earlier lessons. The workstation is an Apple Silicon (arm64) Mac, so the
> VM, the images and the binaries are arm64 too: the deck's `ubuntu/jammy64` box (slide 6) exists
> for amd64 only. The "**" item (Kubernetes) carries no points in the text. The report is the
> "Lesson 10 — додаткова частина: DevOps Basics" section of `README.md`; evidence is under
> `docs/devops/`, screenshots are under `docs/screenshots/devops/`.

**Points:** 70 (Part 1) + 30 (Part 2); Part 3 has no points in the text.

**Objective:** build the deck's DevOps environment around the Albums API — a virtual machine with Docker, the Floci AWS emulator and a verified k3s cluster; run the deck's EC2 and Lambda demos on it; run the application with its database in Docker Compose; deploy it to Kubernetes.

### Original text (verbatim)

```text
Set up an environment consisting of a virtual machine with Docker installed, Flossy running, and a verified K3s deployment. Execute the demos for applications 1 and 2 from the presentation. (70 points)
* Containerize the Go application you created. Use Docker Compose to run it alongside a database. (+30 points)
** Deploy the application in Kubernetes.

Submit screenshots or a video recording demonstrating the application in operation. Also, provide a link to the GitHub repository containing the Docker and Docker Compose configuration files.
```

---

### PART 1: Environment and the deck's demos [70 points]

* **Task 1.1:** a virtual machine with Docker installed, Floci ("Flossy") running and a verified k3s deployment.
* **Task 1.2:** run the demos for applications 1 and 2 from the deck: Demo 1 (EC2 on Floci) and Demo 2 (a Go Lambda on Floci).

---

### PART 2 (*): Docker and Docker Compose [+30 points]

* Containerize the Go application (Albums API) and run it with Docker Compose alongside a database (PostgreSQL).

---

### PART 3 (**): Kubernetes

* Deploy the application to Kubernetes (the k3s cluster from Part 1).

---

### Deliverables

1. Screenshots or a video recording demonstrating the application in operation.
2. A link to the GitHub repository with the Docker and Docker Compose configuration files.
