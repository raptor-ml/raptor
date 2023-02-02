<div id="top"></div>

[![Go Report Card][go-report-card-shield]][go-report-card-url]
[![Go Reference][godoc-shield]][godoc-url]
[![E2E Tests][e2e-tests-shield]][e2e-tests-url]
[![CII Best Practices][best-practices-shield]][best-practices-url]
[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![MIT License][license-shield]][license-url]
[![LinkedIn][linkedin-shield]][linkedin-url]
<!-- [![Contributors][contributors-shield]][contributors-url] -->

<br />
<div align="center">
    <a href="https://raptor.ml">
        <img src=".github/logo.svg" alt="RaptorML - Production-ready feature engineering" width="300">
    </a>

<h3 align="center"><p>From notebook to production</p>Transform your data science to production-ready artifacts</h3>
<br />

  <p align="center">
    Raptor simplifies deploying data science work from a notebook to production; it compiles your python research
    code and takes care of the engineering concerns like scalability and reliability on Kubernetes. Focus on
    the <strong>data science</strong>, RaptorML will take care of the engineering overhead.
    <br />
    <br />
    <a href="https://raptor.ml"><strong>Explore the docs »</strong></a>
    <br />
    <br />
    <a href="https://colab.research.google.com/github/raptor-ml/docs/blob/master/docs/guides/getting-started-with-labsdk.ipynb">Getting started in 5 minutes</a>
    ·
    <a href="https://github.com/raptor-ml/raptor/issues">Report a Bug</a>
    ·
    <a href="https://github.com/raptor-ml/raptor/issues">Request a Feature</a>
  </p>
</div>

[![RaptorML Screen Shot][product-screenshot]][docs-url]

## 🧐 What is Raptor?

Raptor **turns any data scientist into an ML engineer** without learning software engineering.

It's made for applied data scientists and ML engineers who want to build operational models and ML-driven functionality
without the hassle of the infrastructure development, and **focus on the business logic and the research instead.**

With Raptor, data scientists can tweak their Python research code and export it. Then, Raptor will take care of the
production and engineering concerns (such as scale, high availability, authentication, caching, monitoring, etc.)

Once your artifacts deployed to Kubernetes, Raptor take cares of the data-processing and feature calculation in a way
that is optimized for production, deploys the model to model servers such as Sagemaker or Docker containers, connects to
your *production* data sources, and more.

[![Colab][colab-button]][colab-url]

### 😍 Why people *love* Raptor? and how does it change their lives?

Before Raptor, data scientists had to work closely with software engineers to translate their models into production-ready code, connect to data sources, transform their data with Flink/Spark or even Java, create APIs, dockerizing the model, handle scaling and high availability, and more.

With Raptor, data scientists can focus *only* on their research and model development, then export their work to production. Raptor takes care of the rest, including connecting to data sources, transforming the data, deploying and connecting the model, etc. This means data scientists can focus on what they do best, and Raptor handles the rest.

### ⭐️ Key Features

* **Easy to use**<br/>
  Raptor is designed to be easy to use. You can start using it in 5 minutes.
* **Same code for both training and production**<br/>
  You can run the same Raptor compatible features in training and production and prevent the *training serving skew*.
* **Real-Time / On-demand feature calculation**<br/>
  Raptor is optimizing features to be calculated at the time of the request.
* **Caching and storing**<br/>
  Raptor is utilizing an integrated "Reversed Feature-Store" to cache the calculation results and take snapshots of the
  data to cold storage for historical purposes (such as re-training).
* **Compile data science work into production-ready artifacts**<br/>
  Raptor is implementing by-design best-practices functionalities of Kubernetes solutions such as leader-election,
  scaling, health, auto-recovery, monitoring, logging, and more.
* **Connect the ML research to the RND team**<br/>
  Raptor is extending your existing DevOps tools and infrastructure, and it's not replacing them. This way, you can
  connect your ML research to the rest of your organization's R&D ecosystem and utilize the existing tools such as
  CI/CD, monitoring, etc.

<p align="right">(<a href="#top">back to top</a>)</p>

## 💡 How does it work?

The work with Raptor starts in your research phase, in your notebook, or in your favorite IDE. Raptor allows you to
write your ML work in a way that is translatable for production purposes.

Assets in Raptor are composed of a declarative part(via Python's decorators) and a function code. This way, "Raptor
Core" can translate the heavy-lifting engineering concerns(such as aggregations or caching), implement the "declarative
part", and optimize the implementation for production.

![Features are composed from a declarative part and a function code][feature-py-def]

After you are satisfied with the results or your research, "export" these definitions to Kubernetes and deploy them
using standard tools; Once deployed, Raptor Core(the server-side part) is extending Kubernetes with the ability to
implement them. It takes care of the engineering concerns by managing and controlling Kubernetes-native resources such
as deployments to connect your production data sources and run your business logic at scale.

You can read more about Raptor's architecture in [the docs][docs-url].

## ⚡️ Quick start

Raptor's LabSDK is the quickest and most popular way to develop RaptorML compatible features.

[![Colab][colab-button]][colab-url]

The LabSDK allows you to write Raptor-compatible features using Python and "convert" them to Kubernetes resources.
This way, in most of the use-cases, you can iterate and play with your data.

### Production Installation

**Raptor installation is not required for training purposes**.
You only need to install Raptor *when deploying to production* (or staging).

Learn more about production installation at [the docs][docs-url].

#### Prerequisites

1. Kubernetes cluster

   (You can use [Kind](https://kind.sigs.k8s.io/) to install Raptor locally)
2. `kubectl` installed and configured to your cluster.
3. Redis server (> 2.8.9)

### Installation

The easiest way to install Raptor is to use
the [OperatorHub Installation method](https://operatorhub.io/operator/raptor).

<p align="right">(<a href="#top">back to top</a>)</p>

## 🌍 "Hello World" feature

```python
@feature(keys='name')
@freshness(target='-1', invalid_after='-1')
def emails_deals(_, ctx: Context) -> float:
    return f"hello world {ctx.keys['name']}!"
```

## 🐍 Full example

```python
import pandas as pd
from typing_extensions import TypedDict
from labsdk.raptor import *


@data_source(
    training_data=pd.read_csv(
        'https://gist.githubusercontent.com/AlmogBaku/8be77c2236836177b8e54fa8217411f2/raw/hello_world_transactions.csv'),
    keys=['customer_id'],
    production_config=StreamingConfig()
)
class BankTransaction(TypedDict):
    customer_id: int
    amount: float
    timestamp: str


# Define features
@feature(keys='customer_id', data_source=BankTransaction)
@aggregation(function=AggregationFunction.Sum, over='10h', granularity='1h')
def total_spend(this_row: BankTransaction, ctx: Context) -> float:
    """total spend by a customer in the last hour"""
    return this_row['amount']


@feature(keys='customer_id', data_source=BankTransaction)
@freshness(max_age='5h', max_stale='1d')
def amount(this_row: BankTransaction, ctx: Context) -> float:
    """total spend by a customer in the last hour"""
    return this_row['amount']


# Train the model
@model(
    keys=['customer_id'],
    input_features=['total_spend+sum'],
    input_labels=[amount],
    model_framework='sklearn',
    model_server='sagemaker-ack',
)
@freshness(max_age='1h', max_stale='100h')
def amount_prediction(ctx: TrainingContext):
    from sklearn.linear_model import LinearRegression

    df = ctx.features_and_labels()

    trainer = LinearRegression()
    trainer.fit(df[ctx.input_features], df[ctx.input_labels])

    return trainer


amount_prediction.export()
```

Then, we can deploy the generated resources to Kubernetes using `kubectl` or instructing the DevOps team to integrate
the generated `Makefile` into the existing CI/CD pipeline.
<p align="right">(<a href="#top">back to top</a>)</p>



<!-- ROADMAP -->

## 🏔 Roadmap

- [ ] S3 historical storage plugins
    - [x] S3 storing
    - [ ] S3 fetching data - Spark
- [ ] Deploy models to model servers
    - [x] Sagemaker ACK
    - [ ] Seldon
    - [ ] Kubeflow
    - [ ] KFServing
    - [ ] Standalone
- [ ] Large-scale training
- [ ] Support more data sources
    - [x] Kafka
    - [x] GCP Pub/Sub
    - [x] Rest
    - [ ] Snowflake
    - [ ] BigQuery
    - [ ] gRPC
    - [ ] Redis
    - [ ] Postgres
    - [ ] GraphQL

See the [open issues](https://github.com/raptor-ml/raptor/issues) for a full list of proposed features (and known
issues)
.

<p align="right">(<a href="#top">back to top</a>)</p>



<!-- CONTRIBUTING -->

## 👷‍ Contributing

Contributions make the open-source community a fantastic place to learn, inspire, and create. Any contributions you make
are **greatly appreciated** (not only code! but also documenting, blogging, or giving us feedback) 😍.

Please fork the repo and create a pull request if you have a suggestion. You can also simply open an issue and choose "
Feature Request" to give us some feedback.

**Don't forget to give the project a star! ⭐️**

For more information about contributing code to the project, read the [`CONTRIBUTING.md`](./CONTRIBUTING.md) file.

<p align="right">(<a href="#top">back to top</a>)</p>



<!-- LICENSE -->

## 📃 License

Distributed under the Apache2 License. Read the `LICENSE` file for more information.

<p align="right">(<a href="#top">back to top</a>)</p>

## 👫 Joining the community

<p align="right">(<a href="#top">back to top</a>)</p>

[godoc-shield]: https://pkg.go.dev/badge/github.com/raptor-ml/raptor.svg

[godoc-url]: https://pkg.go.dev/github.com/raptor-ml/raptor

[contributors-shield]: https://img.shields.io/github/contributors/raptor-ml/raptor.svg?style=flat

[contributors-url]: https://github.com/raptor-ml/raptor/graphs/contributors

[forks-shield]: https://img.shields.io/github/forks/raptor-ml/raptor.svg?style=flat

[forks-url]: https://github.com/raptor-ml/raptor/network/members

[stars-shield]: https://img.shields.io/github/stars/raptor-ml/raptor.svg?style=flat

[stars-url]: https://github.com/raptor-ml/raptor/stargazers

[issues-shield]: https://img.shields.io/github/issues/raptor-ml/raptor.svg?style=flat

[issues-url]: https://github.com/raptor-ml/raptor/issues

[e2e-tests-shield]: https://img.shields.io/github/workflow/status/raptor-ml/raptor/Integration%20Tests?label=Tests

[e2e-tests-url]: https://github.com/raptor-ml/raptor/actions/workflows/test-e2e.yml

[license-shield]: https://img.shields.io/github/license/raptor-ml/raptor.svg?style=flat

[license-url]: https://github.com/raptor-ml/raptor/blob/master/LICENSE.txt

[linkedin-shield]: https://img.shields.io/badge/-LinkedIn-black.svg?style=flat&logo=linkedin&colorB=555

[linkedin-url]: https://linkedin.com/company/raptor-ml

[go-report-card-shield]: https://goreportcard.com/badge/github.com/raptor-ml/raptor

[go-report-card-url]: https://goreportcard.com/report/github.com/raptor-ml/raptor

[best-practices-shield]: https://bestpractices.coreinfrastructure.org/projects/6406/badge

[best-practices-url]: https://bestpractices.coreinfrastructure.org/projects/6406

[colab-button]: https://img.shields.io/badge/-Getting%20started%20with%20Colab-blue?style=for-the-badge&logo=googlecolab

[colab-url]: https://colab.research.google.com/github/raptor-ml/docs/blob/master/docs/guides/getting-started-with-labsdk.ipynb

[docs-url]: https://raptor.ml/

[product-screenshot]: .github/demo.svg

[feature-py-def]: .github/feature-py-def.png
