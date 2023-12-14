from aws_cdk import (
    aws_logs,
    aws_iam,
    aws_ec2,
    aws_ecs
)

from constructs import Construct

from stack.helper_tags import add_tags

import os

"""
Apukoodit konttien luontiin



"""







"""



"""
class EcsService(Construct):

    def __init__(self,
                 scope: Construct,
                 id: str,
                 path: str,
                 cpu: int,
                 memory_mb: int,
                 vpc: aws_ec2.Vpc,
                 # security_group: aws_ec2.SecurityGroup,
                 project_tag: str,
                 tags: dict = None
                 ):
        super().__init__(scope, id)

        sg = aws_ec2.SecurityGroup(self,
                                   id = f"{id}-SG",
                                   security_group_name = f"{id}-SG",
                                   description = f"{id} outbound only",
                                   vpc = vpc,
                                   allow_all_outbound = True)
        add_tags(sg, tags, project_tag = project_tag)

        cluster = aws_ecs.Cluster(self,
                                  id = f"{id}-cluster",
                                  cluster_name = f"{id}-cluster",
                                  vpc = vpc,
                                  container_insights = True
                                  )
        add_tags(cluster, tags, project_tag = project_tag)

        exec_role = aws_iam.Role(self,
                                 id = f"{id}-execution-role",
                                 role_name = f"{id}-execution-role",
                                 assumed_by = aws_iam.ServicePrincipal("ecs-tasks.amazonaws.com"),
                                 managed_policies = [ 
                                     aws_iam.ManagedPolicy.from_managed_policy_arn(self,
                                                                                   id = "AmazonECSTaskExecutionRolePolicy",
                                                                                   managed_policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
                                                                                   )
                                  ]
                                 )
        add_tags(exec_role, tags, project_tag = project_tag)

        task_role = aws_iam.Role(self,
                                 id = f"{id}-task-role",
                                 role_name = f"{id}-task-role",
                                 assumed_by = aws_iam.ServicePrincipal("ecs-tasks.amazonaws.com"),
                                 managed_policies = [ 
                                     aws_iam.ManagedPolicy.from_managed_policy_arn(self,
                                                                                   id = "fulls3access",
                                                                                   managed_policy_arn = "arn:aws:iam::aws:policy/AmazonS3FullAccess"
                                                                                   ),
                                     aws_iam.ManagedPolicy.from_managed_policy_arn(self,
                                                                                   id = "ssmreadaccess",
                                                                                   managed_policy_arn = "arn:aws:iam::aws:policy/AmazonSSMReadOnlyAccess"
                                                                                   )
                                  ],
                                  inline_policies = {
                                      "secretsreadaccess": aws_iam.PolicyDocument(
                                          statements= [
                                              aws_iam.PolicyStatement(
                                                  effect = aws_iam.Effect.ALLOW,
                                                  actions = [ 
                                                      "secretsmanager:GetSecretValue",
                                                      "secretsmanager:DescribeSecret"
                                                   ],
                                                  resources = [ "arn:aws:secretsmanager:::secret:*" ]
                                              )
                                              
                                          ]
                                      )
                                  }
                                 )
        add_tags(task_role, tags, project_tag = project_tag)


        task = aws_ecs.TaskDefinition(self,
                                      id = f"{id}-task",
                                      compatibility = aws_ecs.Compatibility.FARGATE,
                                      network_mode = aws_ecs.NetworkMode.AWS_VPC,
                                      execution_role = exec_role,
                                      task_role = task_role,
                                      cpu = f"{cpu}",
                                      memory_mib = f"{memory_mb}"
                                      )
        add_tags(task,    tags, project_tag = project_tag)

        basepath = os.path.dirname(os.path.dirname(__file__))
        path = os.path.join(basepath, path)

        task.add_container(id = f"{id}-container",
                           image = aws_ecs.ContainerImage.from_asset(path),
                           cpu = cpu,
                           memory_limit_mib = memory_mb,
                           logging = aws_ecs.LogDrivers.aws_logs(
                               log_retention = aws_logs.RetentionDays.ONE_MONTH,
                               stream_prefix = id
                           ))

        service = aws_ecs.FargateService(self,
                                         id = id,
                                         cluster = cluster,
                                         task_definition = task,
                                         assign_public_ip = False,
                                         desired_count = 1,
                                         security_groups = [ sg ]
                                         )
        add_tags(service, tags, project_tag = project_tag)






