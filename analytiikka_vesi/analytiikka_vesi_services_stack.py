from aws_cdk import (
    Stack,
    aws_ec2,
    aws_s3,
    aws_iam,
    aws_secretsmanager,
    RemovalPolicy
)


from aws_cdk.aws_iam import ServicePrincipal

from constructs import Construct

from analytiikka_vesi.helper_lambda import *
from analytiikka_vesi.helper_glue import *


# TODO: dev/prod parametrien haku jostain



"""
Palvelut stack

"""
class AnalytiikkaVesiServicesStack(Stack):

    def __init__(self,
                 scope: Construct, 
                 construct_id: str,
                 environment: str,
                 **kwargs) -> None:
        super().__init__(scope, construct_id, **kwargs)

        """
        Yhteiset arvot projektilta ja ympäristön mukaan
        """
        properties = self.node.try_get_context(environment)

        # Yhteinen: buketti jdbc- ajureille, glue skripteille jne.
        # Ajurit esim tyypin mukaan omiin polkuihin ( /oracle-driver/ojdbc8.jar, jne )
        script_bucket_name = properties["script_bucket_name"]
        script_bucket = aws_s3.Bucket.from_bucket_name(self, "script-bucket", bucket_name = script_bucket_name)
        # ADE file bucket
        target_bucket_name = properties["ade_staging_bucket_name"]
        # Yhteinen temp- buketti
        temp_bucket_name = properties["temp_bucket_name"]
        # Yhteinen arkisto- buketti
        archive_bucket_name = properties["archive_bucket_name"]
        # Yhteiskäyttöinen rooli lambdoille
        lambda_role_name = self.node.try_get_context('lambda_role_name')
        # Yhteiskäyttöinen securoty group lambdoille. Sallii akiken koska tilin yhteydet on rajattu operaattorin toimesta
        lambda_security_group_name = self.node.try_get_context('lambda_security_group_name')
        # Yhteiskäyttöinen rooli glue- jobeille
        glue_role_name = self.node.try_get_context('glue_role_name')
        # Yhteiskäyttöinen security group glue- jobeille. Sallii akiken koska tilin yhteydet on rajattu operaattorin toimesta
        glue_security_group_name = self.node.try_get_context('glue_security_group_name')

        # print(f"services {environment}: project = '{projectname}'")
        # print(f"services {environment}: account = '{self.account}'")
        # print(f"services {environment}: region = '{self.region}'")
        # print(f"services {environment}: properties = '{properties}'")

        # Lookup: VPC
        vpc_name = properties["vpc_name"]
        vpc = aws_ec2.Vpc.from_lookup(self, "VPC", vpc_name = vpc_name)
        # print(f"services {environment}: vpc = '{vpc}'")
        
        # Lookup: Lambda security group
        lambda_securitygroup = aws_ec2.SecurityGroup.from_lookup_by_name(self, "LambdaSecurityGroup", security_group_name = lambda_security_group_name, vpc = vpc)
        # Lookup: Lambda rooli
        lambda_role = aws_iam.Role.from_role_arn(self, "LambdaRole", f"arn:aws:iam::{self.account}:role/{lambda_role_name}", mutable = False)
        # Lookup: Glue security group
        glue_securitygroup = aws_ec2.SecurityGroup.from_lookup_by_name(self, "GlueSecurityGroup", security_group_name = glue_security_group_name, vpc = vpc)
        # Lookup: Glue rooli
        glue_role = aws_iam.Role.from_role_arn(self, "GlueRole", f"arn:aws:iam::{self.account}:role/{glue_role_name}", mutable = False)




        # HUOM: Lisää tarvittavat tämän jälkeen. Käytä yllä haettuja asioita tarvittaessa (bukettien nimet, roolit, jne)
        #

        # Esimerkki 1 python lambda
        # HUOM: schedule- määritys: https://docs.aws.amazon.com/lambda/latest/dg/services-cloudwatchevents-expressions.html

        # l1 = PythonLambdaFunction(self,
        #                      id = "testi1",
        #                      path = "lambda/testi1",
        #                      index = "testi1.py",
        #                      handler = "testi1.lambda_handler",
        #                      description = "Testilambdan kuvaus",
        #                      role = lambda_role,
        #                      props = LambdaProperties(vpc = vpc,
        #                                               timeout = 2, 
        #                                               environment = {
        #                                                   "target_bucket": target_bucket_name,
        #                                                   "dummy_input_value": "10001101101"
        #                                               },
        #                                               tags = [
        #                                                   { "testitag": "jotain" },
        #                                                   { "toinen": "arvo" }
        #                                               ],
        #                                               securitygroups = [ lambda_securitygroup ],
        #                                               schedule = "0 10 20 * ? *"
        #                                              )
        #                     )


        
        
        
        # l2 = NodejsLambdaFunction(self,
        #                      id = "testi2",
        #                      path = "lambda/testi2",
        #                      handler = "testi2.lambda_handler",
        #                      description = "Testilambdan kuvaus",
        #                      role = lambda_role,
        #                      props = LambdaProperties(vpc = vpc,
        #                                               timeout = 2, 
        #                                               environment = {
        #                                                   "target_bucket": target_bucket_name,
        #                                               },
        #                                               tags = None,
        #                                               securitygroups = [ lambda_securitygroup ],
        #                                               schedule = "0 10 20 * ? *"
        #                                              )
        #                     )



        # glue_sampo_oracle_connection = GlueJdbcConnection(self,
        #                         id = "sampo-jdbc-oracle-connection",
        #                         vpc = vpc,
        #                         security_groups = [ glue_securitygroup ],
        #                         properties = {
        #                             "JDBC_CONNECTION_URL": "jdbc:oracle:thin:@//<host>:<port>/<sid>",
        #                             "JDBC_DRIVER_CLASS_NAME": "oracle.jdbc.driver.OracleDriver",
        #                             "JDBC_DRIVER_JAR_URI": f"s3://{script_bucket_name}/drivers/oracle/ojdbc8.jar",
        #                             "SECRET_ID": f"db-sampo-oracle-{environment}"
        #                         })
        # g1 = PythonSparkGlueJob(self,
        #          id = "testi3", 
        #          path = "glue/testi3",
        #          index = "testi3.py",
        #          script_bucket = script_bucket,
        #          timeout_min = 1,
        #          description = "Glue jobin kuvaus",
        #          worker = "G 1X",
        #          version = None,
        #          role = glue_role,
        #          tags = None,
        #          arguments = None,
        #          connections = [ glue_sampo_oracle_connection.connection ],
        #          enable_spark_ui = False,
        #          schedule = "0 12 24 * ? *",
        #          schedule_description = "Normaali ajastus"
        # )


